#define _WIN32_WINNT 0x0A00 /* win10 */

#include <windows.h>
#include <physicalmonitorenumerationapi.h>

// XXX: lowlevelmonitorconfigurationapi.h is missing in msys2
_BOOL WINAPI SetVCPFeature(HANDLE hMonitor, BYTE bVCPCode, DWORD dwNewValue);

typedef struct {
    BYTE code;
    DWORD value;
    BOOL success;
} VCPRequest;


BOOL enumCallback(HMONITOR hMonitor, HDC hdc, LPRECT lpRect, LPARAM lParam) {
    VCPRequest *req = (VCPRequest*)lParam;
    DWORD numPhysicalMonitors;
    if (!GetNumberOfPhysicalMonitorsFromHMONITOR(hMonitor, &numPhysicalMonitors)) {
        goto fail;
    }
    PHYSICAL_MONITOR *physicalMonitors = calloc(numPhysicalMonitors, sizeof(PHYSICAL_MONITOR));
    if (!physicalMonitors) {
        goto fail;
    }
    if (!GetPhysicalMonitorsFromHMONITOR(hMonitor, numPhysicalMonitors, physicalMonitors)) {
        goto fail;
    }
    for (DWORD i = 0; i < numPhysicalMonitors; i++) {
        SetVCPFeature(physicalMonitors[i].hPhysicalMonitor, req->code, req->value);
        // XXX: we don't check for failures here, since e.g. turning on might not
        // work on some monitors that disable ddc/ci in standby
    }
    if (!DestroyPhysicalMonitors(numPhysicalMonitors, physicalMonitors)) {
        goto fail;
    }
    free(physicalMonitors);
    req->success = TRUE;
    return TRUE;
fail:
    req->success = FALSE;
    return FALSE;
}

BOOL setVCPFeatureAll(BYTE code, DWORD value) {
    VCPRequest req;
    req.code = code;
    req.value = value;
    req.success = FALSE;
    if (!EnumDisplayMonitors(NULL, NULL, (MONITORENUMPROC)enumCallback, (LPARAM)&req)) {
        return FALSE;
    }
    return req.success;
}
