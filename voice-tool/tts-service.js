import { createHash } from "node:crypto";
import { createReadStream, existsSync, mkdirSync, readFileSync, statSync, writeFileSync } from "node:fs";
import { resolve, sep } from "node:path";

const ROOT = process.cwd();
const TTS_CACHE_DIR = resolve(ROOT, process.env.TTS_CACHE_DIR || "audios");
const TTS_MODEL = process.env.OPENAI_TTS_MODEL || "gpt-4o-mini-tts";
const TTS_VOICE = process.env.OPENAI_TTS_VOICE || "cedar";
const TTS_INSTRUCTIONS = process.env.OPENAI_TTS_INSTRUCTIONS || "Speak like a friendly technical co-host. Keep the tone clear, natural, and conversational. Use a steady pace, avoid sounding scripted, and add subtle emphasis to important ideas. Sound helpful, confident, and easy to listen to.";
const TTS_SPEED = normalizeTtsSpeed(process.env.OPENAI_TTS_SPEED || "0.98");

function normalizeTtsSpeed(value) {
  const speed = Number(value || 1);
  if (!Number.isFinite(speed) || speed <= 0) return 1;
  return speed;
}

export function cleanText(value) {
  return String(value || "").replace(/\s+/g, " ").trim();
}

function decodeHtmlEntities(value) {
  return value
    .replace(/&amp;/g, "&")
    .replace(/&lt;/g, "<")
    .replace(/&gt;/g, ">")
    .replace(/&quot;/g, '"')
    .replace(/&#39;/g, "'");
}

function textFromHtml(html) {
  return cleanText(decodeHtmlEntities(html.replace(/<[^>]+>/g, " ")));
}

function markTtsPauses(html) {
  return html.replace(/<span\b[^>]*class="[^"]*\btts-pause\b[^"]*"[^>]*>[\s\S]*?<\/span>/g, (tag) => {
    const seconds = (tag.match(/\bdata-seconds="([^"]+)"/) || [])[1] || "";
    return ` [[TTS_PAUSE:${seconds}]] `;
  });
}

function extractDigiNoteSegments(notesHtml) {
  const digiMatch = notesHtml.match(/<div[^>]*class="[^"]*\bnote-digi\b[^"]*"[^>]*>([\s\S]*?)<\/div>/);
  const markedHtml = markTtsPauses(digiMatch ? digiMatch[1] : notesHtml);
  const segments = [];
  const segmentRe = /\[\[TTS_PAUSE:([^\]]*)\]\]/g;
  let offset = 0;
  let match;

  while ((match = segmentRe.exec(markedHtml)) !== null) {
    const text = textFromHtml(markedHtml.slice(offset, match.index));
    if (text) segments.push({ type: "text", text });

    const seconds = Number(match[1]);
    if (Number.isFinite(seconds) && seconds > 0) {
      segments.push({ type: "pause", seconds });
    }

    offset = segmentRe.lastIndex;
  }

  const finalText = textFromHtml(markedHtml.slice(offset));
  if (finalText) segments.push({ type: "text", text: finalText });

  return segments;
}

function isUsableCachedFile(filePath) {
  try {
    return existsSync(filePath) && statSync(filePath).isFile() && statSync(filePath).size > 0;
  } catch {
    return false;
  }
}

export function displayTtsPath(filePath) {
  return filePath.startsWith(`${ROOT}${sep}`) ? filePath.slice(ROOT.length + 1) : filePath;
}

function ttsCachePath(text) {
  const key = createHash("sha256")
    .update(JSON.stringify({
      model: TTS_MODEL,
      voice: TTS_VOICE,
      instructions: TTS_INSTRUCTIONS,
      speed: TTS_SPEED,
      text,
    }))
    .digest("hex");

  return resolve(TTS_CACHE_DIR, `${key}.mp3`);
}

function supportsTtsInstructions() {
  return TTS_MODEL !== "tts-1" && TTS_MODEL !== "tts-1-hd";
}

function createSpeechPayload(text) {
  const payload = {
    model: TTS_MODEL,
    input: text,
    speed: TTS_SPEED,
    voice: TTS_VOICE,
  };

  if (supportsTtsInstructions() && TTS_INSTRUCTIONS) {
    payload.instructions = TTS_INSTRUCTIONS;
  }

  return payload;
}

export function serveAudioFile(response, filePath, cacheStatus) {
  response.writeHead(200, {
    "Cache-Control": "no-store",
    "Content-Type": "audio/mpeg",
    "X-TTS-Cache": cacheStatus,
  });
  createReadStream(filePath).pipe(response);
}

export async function ensureTtsAudio(text) {
  const filePath = ttsCachePath(text);
  if (isUsableCachedFile(filePath)) {
    console.log(`[tts] cache hit ${displayTtsPath(filePath)}`);
    return { ok: true, filePath, cacheStatus: "hit" };
  }

  const apiKey = process.env.OPENAI_API_KEY;
  if (!apiKey) {
    console.warn(`[tts] missing OPENAI_API_KEY for ${displayTtsPath(filePath)}`);
    return {
      ok: false,
      statusCode: 503,
      payload: { error: "OPENAI_API_KEY not set and audio is not cached" },
    };
  }

  const openAiResponse = await fetch("https://api.openai.com/v1/audio/speech", {
    method: "POST",
    headers: {
      Authorization: `Bearer ${apiKey}`,
      "Content-Type": "application/json",
    },
    body: JSON.stringify(createSpeechPayload(text)),
  });

  if (!openAiResponse.ok) {
    const details = await openAiResponse.text().catch(() => "");
    return {
      ok: false,
      statusCode: 502,
      payload: { error: "OpenAI TTS request failed", status: openAiResponse.status, details },
    };
  }

  const buffer = Buffer.from(await openAiResponse.arrayBuffer());
  mkdirSync(TTS_CACHE_DIR, { recursive: true });
  writeFileSync(filePath, buffer);
  console.log(`[tts] saved ${displayTtsPath(filePath)} (${buffer.length} bytes)`);

  return { ok: true, filePath, cacheStatus: "miss" };
}

export function getSlideVoiceNotes() {
  const html = readFileSync(resolve(ROOT, "index.html"), "utf8");
  const slides = [];
  let index = 0;
  const sectionRe = /<section[^>]*>([\s\S]*?)<\/section>/g;
  let match;

  while ((match = sectionRe.exec(html)) !== null) {
    index += 1;
    const notesHtml = (match[1].match(/<aside[^>]*class="notes"[^>]*>([\s\S]*?)<\/aside>/) || [])[1] || "";
    const segments = extractDigiNoteSegments(notesHtml);

    for (const [part, segment] of segments.entries()) {
      if (segment.type === "text") {
        slides.push({ slide: index, part: part + 1, text: segment.text });
      }
    }
  }

  return slides;
}

export function getTtsConfig() {
  return {
    cacheDir: TTS_CACHE_DIR,
    instructions: supportsTtsInstructions() ? TTS_INSTRUCTIONS : "",
    model: TTS_MODEL,
    speed: TTS_SPEED,
    voice: TTS_VOICE,
  };
}
