// Lightweight i18n. One flat bundle per language; the active language also
// drives the AI response language (sent with every request — see api.ts).
export type Lang = "en" | "hu";

const KEY = "tutor.lang";

export function getStoredLang(): Lang {
  const v = (typeof localStorage !== "undefined" && localStorage.getItem(KEY)) || "";
  return v === "hu" ? "hu" : "en";
}

export function storeLang(lang: Lang) {
  try {
    localStorage.setItem(KEY, lang);
  } catch {
    /* ignore (private mode etc.) */
  }
}

export interface Strings {
  new: string;
  delete: string;
  deleteTitle: string;
  deleteConfirm: string;
  history: string;
  searchLibrary: string;
  noMatches: string;
  noSessions: string;
  recent: string;
  // Spaces
  spaces: string;
  allSpaces: string;
  unfiled: string;
  newSpace: string;
  newSpacePrompt: string;
  deleteSpaceTitle: string;
  deleteSpaceConfirm: string;
  moveTo: string;
  learn: string;
  teach: string;
  teachTitle: string;
  explainMode: string;
  explainModeTitle: string;
  askPlaceholder: string;
  ask: string;
  getFeedback: string;
  explainSubmit: string;
  feynmanIntro: string;
  learnIntro: string;
  explainIntro: string;
  explainSourcePlaceholder: string;
  pasteOpen: string;
  pasteCancel: string;
  pastePlaceholder: string;
  import: string;
  topicPlaceholder: string;
  explainPlaceholder: string;
  // Feynman legend + refine
  feyGap: string;
  feyJargon: string;
  feyShaky: string;
  feyLegendNote: string;
  explainAgainTitle: string;
  explainAgainHint: string;
  explainAgain: string;
  // Thread sheet
  you: string;
  tutor: string;
  askHighlighted: string;
  replyPlaceholder: string;
  newThreadPlaceholder: string;
  send: string;
  on: string;
  actLinkedPage: string;
  actRewrite: string;
  actInsert: string;
  actInsertSummary: string;
  actVisual: string;
  actExercise: string;
  // Selection toolbar
  comment: string;
  textSelected: string;
}

const en: Strings = {
  new: "+ New",
  delete: "Delete",
  deleteTitle: "Delete this session",
  deleteConfirm: "Delete this session? This cannot be undone.",
  history: "History",
  searchLibrary: "Search your library…",
  noMatches: "No matches",
  noSessions: "No sessions yet",
  recent: "Recent",
  spaces: "Spaces",
  allSpaces: "All",
  unfiled: "Unfiled",
  newSpace: "+ New space",
  newSpacePrompt: "Name your new space",
  deleteSpaceTitle: "Delete this space",
  deleteSpaceConfirm: "Delete this space? Its documents stay — they just become unfiled.",
  moveTo: "Move to…",
  learn: "Learn",
  teach: "Teach (Feynman)",
  teachTitle: "Feynman technique: explain it yourself, find your gaps",
  explainMode: "Explain",
  explainModeTitle: "Paste a document — get a one-page explanation of it",
  askPlaceholder: "Ask anything — e.g. how do you calculate acceleration?",
  ask: "Ask",
  getFeedback: "Get feedback",
  explainSubmit: "Explain",
  feynmanIntro:
    "Feynman mode: name a concept and explain it in your own words. Tutor plays a friendly student, points out gaps and undefined jargon, and asks the one question that exposes what's missing — then you explain again.",
  learnIntro:
    "Get a one-page explanation, then select any word, formula, or sentence to ask a follow-up. The AI replies in a thread you can turn into linked pages, rewrites, summaries, visuals, or exercises.",
  explainIntro:
    "Paste a document — a feature file, spec, one-pager, or notes — and Tutor turns it into a one-page explanation, unpacking jargon and condensing where it helps. The result is a normal document you can comment on and explore.",
  explainSourcePlaceholder: "Paste a document, feature file, spec, or notes to explain…",
  pasteOpen: "↑ Paste a .md file instead",
  pasteCancel: "▲ Cancel",
  pastePlaceholder: "Paste your markdown here…",
  import: "Import",
  topicPlaceholder: "What concept are you teaching? e.g. recursion",
  explainPlaceholder: "Explain it in your own words, as if to a curious beginner…",
  feyGap: "vague / missing",
  feyJargon: "undefined jargon",
  feyShaky: "looks shaky",
  feyLegendNote: "highlighted in your own words — tap one to see why",
  explainAgainTitle: "Explain it again",
  explainAgainHint:
    "Fill the gaps and have another go — your explanation gets added below with fresh feedback.",
  explainAgain: "Explain again",
  you: "You",
  tutor: "Tutor",
  askHighlighted: "Ask anything about the highlighted text.",
  replyPlaceholder: "Reply…",
  newThreadPlaceholder: "What do you want to know?",
  send: "Send",
  on: "On",
  actLinkedPage: "↳ Linked page",
  actRewrite: "✎ Rewrite",
  actInsert: "( ) Insert",
  actInsertSummary: "≡ Insert summary",
  actVisual: "◈ Visual",
  actExercise: "✓ Exercise",
  comment: "💬 Comment",
  textSelected: "Text selected",
};

const hu: Strings = {
  new: "+ Új",
  delete: "Törlés",
  deleteTitle: "Munkamenet törlése",
  deleteConfirm: "Törlöd ezt a munkamenetet? Ez nem vonható vissza.",
  history: "Előzmények",
  searchLibrary: "Keresés a könyvtáradban…",
  noMatches: "Nincs találat",
  noSessions: "Még nincs munkamenet",
  recent: "Legutóbbi",
  spaces: "Terek",
  allSpaces: "Mind",
  unfiled: "Besorolatlan",
  newSpace: "+ Új tér",
  newSpacePrompt: "Add meg az új tér nevét",
  deleteSpaceTitle: "Tér törlése",
  deleteSpaceConfirm: "Törlöd ezt a teret? A dokumentumok megmaradnak — csak besorolatlanok lesznek.",
  moveTo: "Áthelyezés…",
  learn: "Tanulás",
  teach: "Taníts (Feynman)",
  teachTitle: "Feynman-technika: magyarázd el te magad, és találd meg a hiányosságokat",
  explainMode: "Elmagyaráz",
  explainModeTitle: "Illessz be egy dokumentumot — kapsz róla egy egyoldalas magyarázatot",
  askPlaceholder: "Kérdezz bármit — pl. hogyan számolod ki a gyorsulást?",
  ask: "Kérdez",
  getFeedback: "Visszajelzést kérek",
  explainSubmit: "Elmagyaráz",
  feynmanIntro:
    "Feynman mód: nevezz meg egy fogalmat, és magyarázd el a saját szavaiddal. A Tutor egy kíváncsi diákot játszik, rámutat a hiányokra és a nem definiált szakszavakra, és felteszi azt az egy kérdést, ami megmutatja, mi hiányzik — aztán újra elmagyarázod.",
  learnIntro:
    "Kapsz egy egyoldalas magyarázatot, majd jelölj ki bármilyen szót, képletet vagy mondatot, hogy rákérdezz. Az AI egy szálban válaszol, amelyből kapcsolt oldalt, átírást, összefoglalót, ábrát vagy gyakorlatot készíthetsz.",
  explainIntro:
    "Illessz be egy dokumentumot — egy feature fájlt, specifikációt, összefoglalót vagy jegyzeteket —, és a Tutor egyoldalas magyarázatot készít belőle, kibontva a szakszavakat és tömörítve, ahol az segít. Az eredmény egy normál dokumentum, amelyhez megjegyzéseket fűzhetsz és felfedezheted.",
  explainSourcePlaceholder: "Illessz be egy dokumentumot, feature fájlt, specifikációt vagy jegyzeteket…",
  pasteOpen: "↑ Inkább illessz be egy .md fájlt",
  pasteCancel: "▲ Mégse",
  pastePlaceholder: "Illeszd be ide a markdownt…",
  import: "Importálás",
  topicPlaceholder: "Milyen fogalmat tanítasz? pl. rekurzió",
  explainPlaceholder: "Magyarázd el a saját szavaiddal, mintha egy kíváncsi kezdőnek mondanád…",
  feyGap: "homályos / hiányzó",
  feyJargon: "nem definiált szakszó",
  feyShaky: "bizonytalannak tűnik",
  feyLegendNote: "a saját szavaiddal kiemelve — koppints rá, hogy lásd, miért",
  explainAgainTitle: "Magyarázd el újra",
  explainAgainHint:
    "Töltsd ki a hiányokat, és próbáld újra — a magyarázatod alább megjelenik friss visszajelzéssel.",
  explainAgain: "Magyarázd el újra",
  you: "Te",
  tutor: "Tutor",
  askHighlighted: "Kérdezz bármit a kiemelt szövegről.",
  replyPlaceholder: "Válasz…",
  newThreadPlaceholder: "Mit szeretnél megtudni?",
  send: "Küldés",
  on: "Erről:",
  actLinkedPage: "↳ Kapcsolt oldal",
  actRewrite: "✎ Átírás",
  actInsert: "( ) Beszúrás",
  actInsertSummary: "≡ Összefoglaló beszúrása",
  actVisual: "◈ Ábra",
  actExercise: "✓ Gyakorlat",
  comment: "💬 Megjegyzés",
  textSelected: "Szöveg kijelölve",
};

const STRINGS: Record<Lang, Strings> = { en, hu };

export function strings(lang: Lang): Strings {
  return STRINGS[lang];
}
