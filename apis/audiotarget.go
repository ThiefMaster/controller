package apis

import (
	"errors"
	"log"
	"syscall"
	"unsafe"

	"github.com/go-ole/go-ole"
	"github.com/moutend/go-wca/pkg/wca"
)

var (
	IID_IPolicyConfigVista  = ole.NewGUID("568b9108-44bf-40b4-9006-86afe5b5a620")
	CLSID_PolicyConfigVista = ole.NewGUID("294935CE-F637-4E7C-A41B-AB255460B862")
)

type IPolicyConfigVista struct {
	ole.IUnknown
}

type IPolicyConfigVistaVtbl struct {
	ole.IUnknownVtbl
	GetMixFormat          uintptr
	GetDeviceFormat       uintptr
	SetDeviceFormat       uintptr
	GetProcessingPeriod   uintptr
	SetProcessingPeriod   uintptr
	GetShareMode          uintptr
	SetShareMode          uintptr
	GetPropertyValue      uintptr
	SetPropertyValue      uintptr
	SetDefaultEndpoint    uintptr
	SetEndpointVisibility uintptr
}

func (v *IPolicyConfigVista) VTable() *IPolicyConfigVistaVtbl {
	return (*IPolicyConfigVistaVtbl)(unsafe.Pointer(v.RawVTable))
}

func (v *IPolicyConfigVista) SetDefaultEndpoint(deviceID string, eRole wca.ERole) (err error) {
	err = pcvSetDefaultEndpoint(v, deviceID, eRole)
	return
}

func pcvSetDefaultEndpoint(pcv *IPolicyConfigVista, deviceID string, eRole wca.ERole) (err error) {
	var ptr *uint16
	if ptr, err = syscall.UTF16PtrFromString(deviceID); err != nil {
		return
	}
	hr, _, _ := syscall.Syscall(
		pcv.VTable().SetDefaultEndpoint,
		3,
		uintptr(unsafe.Pointer(pcv)),
		uintptr(unsafe.Pointer(ptr)),
		uintptr(uint32(eRole)))
	if hr != 0 {
		err = ole.NewError(hr)
	}
	return
}

// see https://github.com/moutend/go-wca/issues/8 & https://github.com/moutend/go-wca/pull/9
func mmdGetID(mmd *wca.IMMDevice, strID *string) (err error) {
	var strIDPtr uint64
	hr, _, _ := syscall.Syscall(
		mmd.VTable().GetId,
		2,
		uintptr(unsafe.Pointer(mmd)),
		uintptr(unsafe.Pointer(&strIDPtr)),
		0)
	if hr != 0 {
		err = ole.NewError(hr)
		return
	}
	// According to the MSDN document, an endpoint ID string is a null-terminated wide-character string.
	// https://msdn.microsoft.com/en-us/library/windows/desktop/dd370837(v=vs.85).aspx
	var us []uint16
	var i uint32
	var start = unsafe.Pointer(uintptr(strIDPtr))
	for {
		u := *(*uint16)(unsafe.Pointer(uintptr(start) + 2*uintptr(i)))
		if u == 0 {
			break
		}
		us = append(us, u)
		i++
	}
	*strID = syscall.UTF16ToString(us)
	ole.CoTaskMemFree(uintptr(strIDPtr))
	return
}

func getDeviceShortName(mmd *wca.IMMDevice) (name string, err error) {
	var ps *wca.IPropertyStore
	if err := mmd.OpenPropertyStore(wca.STGM_READ, &ps); err != nil {
		return "", err
	}
	defer ps.Release()

	var pv wca.PROPVARIANT
	if err := ps.GetValue(&wca.PKEY_Device_FriendlyName, &pv); err != nil {
		return "", err
	}

	return pv.String(), nil
}

func SetNextDefaultEndpoint() (err error) {
	err = ole.CoInitializeEx(0, ole.COINIT_APARTMENTTHREADED)
	if err != nil {
		return err
	}
	defer ole.CoUninitialize()

	var mmde *wca.IMMDeviceEnumerator
	if err = wca.CoCreateInstance(wca.CLSID_MMDeviceEnumerator, 0, wca.CLSCTX_ALL, wca.IID_IMMDeviceEnumerator, &mmde); err != nil {
		return err
	}
	defer mmde.Release()

	var mmd *wca.IMMDevice
	if err = mmde.GetDefaultAudioEndpoint(wca.ERender, wca.EConsole, &mmd); err != nil {
		return err
	}
	defer mmd.Release()

	var defaultDevID string
	if err = mmdGetID(mmd, &defaultDevID); err != nil {
		return err
	}

	var dco *wca.IMMDeviceCollection
	if err = mmde.EnumAudioEndpoints(wca.ERender, wca.DEVICE_STATE_ACTIVE, &dco); err != nil {
		return err
	}

	var count uint32
	if err = dco.GetCount(&count); err != nil {
		return err
	}

	var ids []string
	devMap := make(map[string]string)
	for i := uint32(0); i < count; i++ {
		var mmd *wca.IMMDevice
		if err = dco.Item(i, &mmd); err != nil {
			continue
		}
		var id string
		if err = mmdGetID(mmd, &id); err != nil {
			continue
		}
		ids = append(ids, id)

		var name string
		if name, err = getDeviceShortName(mmd); err != nil {
			return err
		}
		devMap[id] = name
	}

	var next string
	for i, id := range ids {
		if id == defaultDevID {
			next = ids[(i+1)%len(ids)]
			break
		}
	}

	if next == "" || next == defaultDevID {
		return errors.New("No alternative device found")
	}

	log.Printf("Switching default audio output to: %s\n", devMap[next])

	var pcv *IPolicyConfigVista
	if err = wca.CoCreateInstance(CLSID_PolicyConfigVista, 0, wca.CLSCTX_ALL, IID_IPolicyConfigVista, &pcv); err != nil {
		return err
	}
	defer pcv.Release()

	if err = pcv.SetDefaultEndpoint(next, wca.EConsole); err != nil {
		return err
	}

	return nil
}
