// based on https://github.com/bbigras/go-windows-session-notifications

#define _WIN32_WINNT 0x0A00 /* win10 */

#include <windows.h>
#include <wtsapi32.h>
#include "_cgo_export.h"


LRESULT CALLBACK WndProc(HWND hWnd, UINT message, WPARAM wParam, LPARAM lParam)
{
	int wmId, wmEvent;
	if (message == WM_WTSSESSION_CHANGE) {
		relayMessage(wParam);
		return 0;
	} else {
		return DefWindowProc(hWnd, message, wParam, lParam);
	}
}


DWORD WINAPI WatchSessionNotifications(LPVOID lpParam)
{
	char const *lpClassName = "WTSMonitor";

	WNDCLASS wc;
	wc.lpfnWndProc = WndProc;
	wc.lpszClassName = lpClassName;
	if (!RegisterClass(&wc)) {
		return 0;
	}

	HWND hwnd = CreateWindow(
		lpClassName, "WTS Monitor", 0, CW_USEDEFAULT, CW_USEDEFAULT, 0, 0, HWND_MESSAGE, NULL, NULL, NULL);
	if (!hwnd) {
		return 0;
	}

	UpdateWindow(hwnd);
	WTSRegisterSessionNotification(hwnd, NOTIFY_FOR_THIS_SESSION);

	MSG msg;
	while (GetMessage(&msg, NULL, 0, 0) > 0) {
		TranslateMessage(&msg);
		DispatchMessage(&msg);
	}
}


void Start() {
	DWORD lpThreadId, lpParameter = 1;
	CreateThread(NULL, 0, WatchSessionNotifications, &lpParameter, 0, &lpThreadId);
}
