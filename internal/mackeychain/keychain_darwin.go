//go:build darwin && cgo

// Package mackeychain provides the narrow Data Protection Keychain operations
// used by Alzette Connect. It deliberately does not expose access groups,
// synchronisation, authentication prompts, or arbitrary query attributes.
package mackeychain

/*
#cgo LDFLAGS: -framework CoreFoundation -framework Security

#include <CoreFoundation/CoreFoundation.h>
#include <Security/Security.h>
#include <stdlib.h>
#include <string.h>

static CFStringRef alz_string(const char *value) {
	return CFStringCreateWithCString(kCFAllocatorDefault, value, kCFStringEncodingUTF8);
}

static CFMutableDictionaryRef alz_query(const char *service, const char *account) {
	CFStringRef serviceValue = alz_string(service);
	CFStringRef accountValue = alz_string(account);
	if (serviceValue == NULL || accountValue == NULL) {
		if (serviceValue != NULL) CFRelease(serviceValue);
		if (accountValue != NULL) CFRelease(accountValue);
		return NULL;
	}

	CFMutableDictionaryRef query = CFDictionaryCreateMutable(
		kCFAllocatorDefault, 0, &kCFTypeDictionaryKeyCallBacks,
		&kCFTypeDictionaryValueCallBacks);
	if (query != NULL) {
		CFDictionarySetValue(query, kSecClass, kSecClassGenericPassword);
		CFDictionarySetValue(query, kSecAttrService, serviceValue);
		CFDictionarySetValue(query, kSecAttrAccount, accountValue);
		CFDictionarySetValue(query, kSecUseDataProtectionKeychain, kCFBooleanTrue);
	}
	CFRelease(serviceValue);
	CFRelease(accountValue);
	return query;
}

static OSStatus alz_keychain_add(
	const char *service, const char *account,
	const unsigned char *value, size_t valueLength) {
	CFMutableDictionaryRef query = alz_query(service, account);
	if (query == NULL) return errSecAllocate;

	CFDataRef data = CFDataCreate(kCFAllocatorDefault, value, (CFIndex)valueLength);
	CFStringRef label = alz_string("Alzette Connect");
	if (data == NULL || label == NULL) {
		if (data != NULL) CFRelease(data);
		if (label != NULL) CFRelease(label);
		CFRelease(query);
		return errSecAllocate;
	}
	CFDictionarySetValue(query, kSecValueData, data);
	CFDictionarySetValue(query, kSecAttrLabel, label);
	CFDictionarySetValue(query, kSecAttrAccessible,
		kSecAttrAccessibleWhenUnlockedThisDeviceOnly);

	OSStatus status = SecItemAdd(query, NULL);
	CFRelease(label);
	CFRelease(data);
	CFRelease(query);
	return status;
}

static OSStatus alz_keychain_update(
	const char *service, const char *account,
	const unsigned char *value, size_t valueLength) {
	CFMutableDictionaryRef query = alz_query(service, account);
	if (query == NULL) return errSecAllocate;

	CFDataRef data = CFDataCreate(kCFAllocatorDefault, value, (CFIndex)valueLength);
	CFMutableDictionaryRef changes = CFDictionaryCreateMutable(
		kCFAllocatorDefault, 0, &kCFTypeDictionaryKeyCallBacks,
		&kCFTypeDictionaryValueCallBacks);
	if (data == NULL || changes == NULL) {
		if (data != NULL) CFRelease(data);
		if (changes != NULL) CFRelease(changes);
		CFRelease(query);
		return errSecAllocate;
	}
	CFDictionarySetValue(changes, kSecValueData, data);

	OSStatus status = SecItemUpdate(query, changes);
	CFRelease(changes);
	CFRelease(data);
	CFRelease(query);
	return status;
}

static OSStatus alz_keychain_copy(
	const char *service, const char *account,
	unsigned char **value, size_t *valueLength, size_t maximumLength) {
	*value = NULL;
	*valueLength = 0;
	CFMutableDictionaryRef query = alz_query(service, account);
	if (query == NULL) return errSecAllocate;
	CFDictionarySetValue(query, kSecMatchLimit, kSecMatchLimitOne);
	CFDictionarySetValue(query, kSecReturnData, kCFBooleanTrue);

	CFTypeRef result = NULL;
	OSStatus status = SecItemCopyMatching(query, &result);
	CFRelease(query);
	if (status != errSecSuccess) return status;
	if (result == NULL || CFGetTypeID(result) != CFDataGetTypeID()) {
		if (result != NULL) CFRelease(result);
		return errSecDecode;
	}

	CFDataRef data = (CFDataRef)result;
	CFIndex length = CFDataGetLength(data);
	if (length <= 0 || (size_t)length > maximumLength) {
		CFRelease(result);
		return errSecDecode;
	}
	unsigned char *copy = malloc((size_t)length);
	if (copy == NULL) {
		CFRelease(result);
		return errSecAllocate;
	}
	memcpy(copy, CFDataGetBytePtr(data), (size_t)length);
	CFRelease(result);
	*value = copy;
	*valueLength = (size_t)length;
	return errSecSuccess;
}

static OSStatus alz_keychain_delete(const char *service, const char *account) {
	CFMutableDictionaryRef query = alz_query(service, account);
	if (query == NULL) return errSecAllocate;
	OSStatus status = SecItemDelete(query);
	CFRelease(query);
	return status;
}
*/
import "C"

import (
	"errors"
	"fmt"
	"strings"
	"unsafe"
)

const maximumValueLength = 64 << 10

var ErrNotFound = errors.New("macOS Keychain item not found")

type StatusError struct {
	Operation string
	Status    int32
}

func (e *StatusError) Error() string {
	return fmt.Sprintf("macOS Keychain %s failed (OSStatus %d)", e.Operation, e.Status)
}

func Get(service, account string) ([]byte, error) {
	serviceValue, accountValue, release, err := identifiers(service, account)
	if err != nil {
		return nil, err
	}
	defer release()
	var value *C.uchar
	var valueLength C.size_t
	status := C.alz_keychain_copy(
		serviceValue, accountValue, &value, &valueLength, C.size_t(maximumValueLength))
	if status != C.errSecSuccess {
		return nil, statusError("read", status)
	}
	defer C.free(unsafe.Pointer(value))
	return C.GoBytes(unsafe.Pointer(value), C.int(valueLength)), nil
}

func Set(service, account string, value []byte) error {
	if len(value) == 0 || len(value) > maximumValueLength {
		return errors.New("macOS Keychain value length is invalid")
	}
	serviceValue, accountValue, release, err := identifiers(service, account)
	if err != nil {
		return err
	}
	defer release()
	data := C.CBytes(value)
	defer C.free(data)
	status := C.alz_keychain_add(
		serviceValue, accountValue, (*C.uchar)(data), C.size_t(len(value)))
	if status == C.errSecDuplicateItem {
		status = C.alz_keychain_update(
			serviceValue, accountValue, (*C.uchar)(data), C.size_t(len(value)))
	}
	if status != C.errSecSuccess {
		return statusError("write", status)
	}
	return nil
}

func Delete(service, account string) error {
	serviceValue, accountValue, release, err := identifiers(service, account)
	if err != nil {
		return err
	}
	defer release()
	status := C.alz_keychain_delete(serviceValue, accountValue)
	if status == C.errSecItemNotFound || status == C.errSecSuccess {
		return nil
	}
	return statusError("delete", status)
}

func identifiers(service, account string) (*C.char, *C.char, func(), error) {
	if service == "" || account == "" || len(service) > 512 || len(account) > 512 ||
		strings.IndexByte(service, 0) >= 0 || strings.IndexByte(account, 0) >= 0 {
		return nil, nil, func() {}, errors.New("macOS Keychain identifier is invalid")
	}
	serviceValue, accountValue := C.CString(service), C.CString(account)
	return serviceValue, accountValue, func() {
		C.free(unsafe.Pointer(serviceValue))
		C.free(unsafe.Pointer(accountValue))
	}, nil
}

func statusError(operation string, status C.OSStatus) error {
	if status == C.errSecItemNotFound {
		return ErrNotFound
	}
	return &StatusError{Operation: operation, Status: int32(status)}
}
