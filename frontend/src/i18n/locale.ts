export type AppLocale = "ru" | "en";

/** Russian UI when any preferred OS/browser language starts with `ru`; otherwise English. */
export function detectLocale(): AppLocale {
  const langs =
    typeof navigator !== "undefined" && navigator.languages?.length
      ? navigator.languages
      : typeof navigator !== "undefined" && navigator.language
        ? [navigator.language]
        : ["en"];
  for (const raw of langs) {
    const base = (raw || "").split("-")[0].toLowerCase();
    if (base === "ru") return "ru";
  }
  return "en";
}
