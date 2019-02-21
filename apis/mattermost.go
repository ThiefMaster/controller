package apis

import (
	"log"
	"strings"
	"time"

	mm "github.com/mattermost/mattermost-server/model"
)

type MattermostSettings struct {
	ServerURL   string `yaml:"url"`
	AccessToken string `yaml:"token"`
	TeamName    string `yaml:"team"`
	ChannelName string `yaml:"channel"`
}

type MattermostState struct {
	HasMessages bool
	HasMentions bool
}

func SubscribeMattermostState(settings MattermostSettings) <-chan MattermostState {
	eventChan := make(chan MattermostState)
	go subscribeMattermostState(eventChan, settings)
	return eventChan
}

func retry(eventChan chan<- MattermostState, settings MattermostSettings) {
	time.Sleep(1 * time.Second)
	subscribeMattermostState(eventChan, settings)
}

func subscribeMattermostState(eventChan chan<- MattermostState, settings MattermostSettings) {
	defer retry(eventChan, settings)

	client := mm.NewAPIv4Client(settings.ServerURL)
	client.AuthType = mm.HEADER_TOKEN
	client.AuthToken = settings.AccessToken

	var userId, channelId string

	if me, resp := client.GetMe(""); resp.Error != nil {
		log.Printf("could not get user info from mattermost: %v\n", resp.Error)
		return
	} else {
		userId = me.Id
	}

	if channel, resp := client.GetChannelByNameForTeamName(settings.ChannelName, settings.TeamName, ""); resp.Error != nil {
		log.Printf("could not get channel from mattermost: %v\n", resp.Error)
		return
	} else {
		channelId = channel.Id
	}

	messageChannels := make(map[string]bool)
	mentionChannels := make(map[string]bool)
	getCurrentUnreads(settings, client, channelId, messageChannels, mentionChannels)
	state := MattermostState{
		HasMessages: len(messageChannels) > 0,
		HasMentions: len(mentionChannels) > 0,
	}
	eventChan <- state

	// connect to websocket for live updates
	var ws *mm.WebSocketClient
	var err *mm.AppError
	if ws, err = mm.NewWebSocketClient(strings.Replace(settings.ServerURL, "http", "ws", 1), client.AuthToken); err != nil {
		log.Printf("could not connect to websocket: %v\n", err)
		return
	}
	ws.Listen()
	for {
		select {
		case <-ws.PingTimeoutChannel:
			log.Println("mattermost websocket: ping timeout")
			ws.Close()
			return
		case resp := <-ws.EventChannel:
			if resp == nil {
				log.Println("mattermost websocket: event channel closed")
				ws.Close()
				return
			}
			if resp.Event == mm.WEBSOCKET_EVENT_CHANNEL_VIEWED {
				delete(messageChannels, resp.Data["channel_id"].(string))
				delete(mentionChannels, resp.Data["channel_id"].(string))
				newState := MattermostState{
					HasMessages: len(messageChannels) > 0,
					HasMentions: len(mentionChannels) > 0,
				}
				if newState != state {
					eventChan <- newState
					state = newState
				}
			} else if resp.Event == mm.WEBSOCKET_EVENT_POSTED {
				post := mm.PostFromJson(strings.NewReader(resp.Data["post"].(string)))
				isDirect := resp.Data["channel_type"] == mm.CHANNEL_DIRECT || resp.Data["channel_type"] == mm.CHANNEL_GROUP
				if post.UserId != userId && (post.ChannelId == channelId || isDirect) {
					messageChannels[post.ChannelId] = true
					if resp.Data["mentions"] != nil {
						mentions := mm.ArrayFromJson(strings.NewReader(resp.Data["mentions"].(string)))
						for _, v := range mentions {
							if v == userId {
								mentionChannels[post.ChannelId] = true
								break
							}
						}
					}
					newState := MattermostState{
						HasMessages: len(messageChannels) > 0,
						HasMentions: len(mentionChannels) > 0,
					}
					if newState != state {
						eventChan <- newState
						state = newState
					}
				}
			}
		}
	}
}

func getCurrentUnreads(
	settings MattermostSettings, client *mm.Client4,
	channelId string,
	messageChannels, mentionChannels map[string]bool,
) {
	// get team id
	var teamId string
	if team, resp := client.GetTeamByName(settings.TeamName, ""); resp.Error != nil {
		log.Printf("could not get team from mattermost: %v\n", resp.Error)
		return
	} else {
		teamId = team.Id
	}

	// get channel details
	channelsById := make(map[string]*mm.Channel)
	if channels, resp := client.GetChannelsForTeamForUser(teamId, "me", ""); resp.Error != nil {
		log.Printf("could not get channels from mattermost: %v\n", resp.Error)
		return
	} else {
		for _, channel := range channels {
			channelsById[channel.Id] = channel
		}
	}

	// get own channel membership, which includes the unread counts
	if members, resp := client.GetChannelMembersForUser("me", teamId, ""); resp.Error != nil {
		log.Printf("could not get unreads from mattermost: %v\n", resp.Error)
	} else {
		for _, member := range *members {
			channel := channelsById[member.ChannelId]
			if channel == nil {
				// channels is in different team
				continue
			}

			if channel.Id == channelId || channel.IsGroupOrDirect() {
				unreadCount := channel.TotalMsgCount - member.MsgCount
				if unreadCount > 0 {
					messageChannels[member.ChannelId] = true
				}
				if member.MentionCount > 0 {
					mentionChannels[member.ChannelId] = true
				}
			}
		}
	}
}
