package i18n

import "fmt"

func ErrSendToSelf() error {
	return fmt.Errorf("%s", Pick(
		"нельзя отправить файлы самому себе",
		"cannot send files to yourself",
	))
}

func ErrPairSelf() error {
	return fmt.Errorf("%s", Pick(
		"нельзя подключиться к своему приглашению",
		"cannot pair with your own invite",
	))
}

func ErrDeviceNotFound(id string) error {
	return fmt.Errorf("%s: %s", Pick("устройство не найдено", "device not found"), id)
}

func ErrDeviceNotFoundShort() error {
	return fmt.Errorf("%s", Pick("устройство не найдено", "device not found"))
}

func ErrPeerNoAddress(name string) error {
	return fmt.Errorf("%s %q %s", Pick("у устройства", "device"), name, Pick("нет адреса для подключения", "has no address to connect"))
}

func ErrPeerNoFingerprint(name string) error {
	return fmt.Errorf("%s %q %s", Pick("у устройства", "device"), name, Pick("нет отпечатка TLS", "has no TLS fingerprint"))
}

func ErrEmptyMessage() error {
	return fmt.Errorf("%s", Pick("пустое сообщение", "empty message"))
}

func ErrMessageTooLong(max int) error {
	return fmt.Errorf("%s (%s %d %s)", Pick("сообщение слишком длинное", "message is too long"), Pick("макс", "max"), max, Pick("байт", "bytes"))
}

func ErrMessageNotUTF8() error {
	return fmt.Errorf("%s", Pick("сообщение должно быть корректным UTF-8", "message must be valid UTF-8"))
}

func ErrChatSend(err error) error {
	return fmt.Errorf("%s: %v", Pick("не удалось отправить", "could not send"), err)
}

func ErrChatRejected(code int) error {
	return fmt.Errorf("%s (HTTP %d)", Pick("получатель отклонил сообщение", "recipient rejected the message"), code)
}
