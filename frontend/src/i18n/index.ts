import en, { type MessageKey } from "./en";
import ru from "./ru";
import { detectLocale, type AppLocale } from "./locale";

export const locale: AppLocale = detectLocale();

const dict = locale === "ru" ? ru : en;

export function t(key: MessageKey, params?: Record<string, string | number>): string {
  let s: string = dict[key] ?? en[key] ?? key;
  if (params) {
    for (const [k, v] of Object.entries(params)) {
      s = s.split(`{${k}}`).join(String(v));
    }
  }
  return s;
}

/** Map backend / API error text to the active UI locale when possible. */
export function localizeError(message: string): string {
  if (!message || locale === "ru") return message;
  const map: Record<string, string> = {
    "нельзя отправить файлы самому себе": "Cannot send files to yourself",
    "устройство не найдено": "Device not found",
    "у устройства": "Device", // prefix for several messages
    "нет адреса для подключения": "has no address to connect",
    "нет отпечатка TLS": "has no TLS fingerprint",
    "пустое сообщение": "Empty message",
    "сообщение слишком длинное": "Message is too long",
    "сообщение должно быть корректным UTF-8": "Message must be valid UTF-8",
    "не удалось отправить": "Could not send",
    "получатель отклонил сообщение": "Recipient rejected the message",
    "downloads folder is not available": "Downloads folder is not available",
    "слишком много файлов": "too many files",
    "нет файлов": "no files",
    "Ожидание подтверждения": "Waiting for confirmation",
    "Получатель отклонил передачу": "Recipient declined the transfer",
    "Получатель занят другой передачей": "Recipient is busy with another transfer",
    "Получатель не ответил вовремя": "Recipient did not respond in time",
    "Не удалось связаться": "Could not connect",
    "Время ожидания истекло": "Timed out",
    "Приложение закрывается": "Application is closing",
    "Отменено": "Canceled",
    "Готово": "Done",
    "Отправлено": "Sent",
    "Файлы сохранены в": "Files saved to",
    "cannot send files to yourself": "Cannot send files to yourself",
    "device not found": "Device not found",
    "has no address to connect": "has no address to connect",
    "has no TLS fingerprint": "has no TLS fingerprint",
    "empty message": "Empty message",
    "message is too long": "Message is too long",
    "message must be valid UTF-8": "Message must be valid UTF-8",
    "could not send": "Could not send",
    "recipient rejected the message": "Recipient rejected the message",
  };
  let out = message;
  for (const [ruText, enText] of Object.entries(map)) {
    if (out.includes(ruText)) out = out.split(ruText).join(enText);
  }
  return out;
}

export function discoveryLabelFor(peerCount: number): string {
  if (peerCount === 0) return t("discovery.searching");
  if (peerCount === 1) return t("discovery.oneNearby");
  return t("discovery.manyNearby", { n: peerCount });
}

export function folderCountLabel(n: number): string {
  return n === 1 ? t("transfer.foldersOne", { n }) : t("transfer.foldersMany", { n });
}

export { detectLocale, type AppLocale, type MessageKey };
