package apis

import (
	"context"
	"encoding/json"
	"log"
	"strings"
	"time"

	mm "github.com/mattermost/mattermost/server/public/model"
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
	client.SetToken(settings.AccessToken)

	var userId, channelId string

	if me, _, err := client.GetMe(context.Background(), ""); err != nil {
		log.Printf("could not get user info from mattermost: %v\n", err)
		return
	} else {
		userId = me.Id
	}

	if channel, _, err := client.GetChannelByNameForTeamName(context.Background(), settings.ChannelName, settings.TeamName, ""); err != nil {
		log.Printf("could not get channel from mattermost: %v\n", err)
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
	ws, err := mm.NewWebSocketClient(strings.Replace(settings.ServerURL, "http", "ws", 1), client.AuthToken)
	if err != nil {
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
			if resp.EventType() == mm.WebsocketEventChannelViewed {
				// TODO remove, not used by recent mattermost versions
				log.Printf("mattermost websocket: received obsolete channel_viewed event: %v\n", resp.GetData())
				delete(messageChannels, resp.GetData()["channel_id"].(string))
				delete(mentionChannels, resp.GetData()["channel_id"].(string))
				newState := MattermostState{
					HasMessages: len(messageChannels) > 0,
					HasMentions: len(mentionChannels) > 0,
				}
				if newState != state {
					eventChan <- newState
					state = newState
				}
			} else if resp.EventType() == mm.WebsocketEventMultipleChannelsViewed {
				for channelId := range resp.GetData()["channel_times"].(map[string]interface{}) {
					delete(messageChannels, channelId)
					delete(mentionChannels, channelId)
					newState := MattermostState{
						HasMessages: len(messageChannels) > 0,
						HasMentions: len(mentionChannels) > 0,
					}
					if newState != state {
						eventChan <- newState
						state = newState
					}
				}
			} else if resp.EventType() == mm.WebsocketEventPosted {
				log.Printf("post: %s\n", resp.GetData())
				var post mm.Post
				if err := json.Unmarshal([]byte(resp.GetData()["post"].(string)), &post); err != nil {
					log.Println("mattermost websocket: could not unmarshal post")
					return
				}
				channelType := mm.ChannelType(resp.GetData()["channel_type"].(string))
				isDirect := channelType == mm.ChannelTypeDirect || channelType == mm.ChannelTypeGroup
				if post.UserId != userId && (post.ChannelId == channelId || isDirect) {
					messageChannels[post.ChannelId] = true
					if resp.GetData()["mentions"] != nil {
						mentions := mm.ArrayFromJSON(strings.NewReader(resp.GetData()["mentions"].(string)))
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
	if team, _, err := client.GetTeamByName(context.Background(), settings.TeamName, ""); err != nil {
		log.Printf("could not get team from mattermost: %v\n", err)
		return
	} else {
		teamId = team.Id
	}

	// get channel details
	channelsById := make(map[string]*mm.Channel)
	if channels, _, err := client.GetChannelsForTeamForUser(context.Background(), teamId, "me", false, ""); err != nil {
		log.Printf("could not get channels from mattermost: %v\n", err)
		return
	} else {
		for _, channel := range channels {
			channelsById[channel.Id] = channel
		}
	}

	// get own channel membership, which includes the unread counts
	if members, _, err := client.GetChannelMembersForUser(context.Background(), "me", teamId, ""); err != nil {
		log.Printf("could not get unreads from mattermost: %v\n", err)
	} else {
		for _, member := range members {
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
