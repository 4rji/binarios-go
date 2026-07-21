export function getNotesPlaybackItems(slide) {
  const notes = slide && slide.querySelector("aside.notes");
  if (!notes) return [];

  const source = notes.querySelector(".note-digi") || notes;
  const items = [];
  const textParts = [];

  const flushText = () => {
    const text = textParts.join(" ").replace(/\s+/g, " ").trim();
    textParts.length = 0;
    if (text) items.push({ type: "text", text });
  };

  const walk = (node) => {
    if (node.nodeType === Node.TEXT_NODE) {
      textParts.push(node.nodeValue);
      return;
    }

    if (node.nodeType !== Node.ELEMENT_NODE) return;

    if (node.matches(".tts-pause")) {
      flushText();
      const seconds = Number(node.dataset.seconds || 0);
      if (Number.isFinite(seconds) && seconds > 0) {
        items.push({ type: "pause", seconds });
      }
      return;
    }

    for (const child of node.childNodes) walk(child);
    if (["P", "DIV", "BR"].includes(node.tagName)) textParts.push(" ");
  };

  walk(source);
  flushText();
  return items;
}

export function createTtsPlayer({ getCurrentSlide, initialDelayMs = 0 }) {
  let audio = null;
  let audioUrl = "";
  let abortController = null;
  let preloadPromise = null;

  function releaseAudio() {
    if (audio) {
      audio.pause();
      audio.currentTime = 0;
      audio = null;
    }

    if (audioUrl) {
      URL.revokeObjectURL(audioUrl);
      audioUrl = "";
    }
  }

  function stop() {
    if (abortController) {
      abortController.abort();
      abortController = null;
    }
    releaseAudio();
  }

  function waitForPause(ms, controller) {
    return new Promise((resolve) => {
      if (controller.signal.aborted) {
        resolve();
        return;
      }

      const timer = window.setTimeout(resolve, ms);
      controller.signal.addEventListener("abort", () => {
        window.clearTimeout(timer);
        resolve();
      }, { once: true });
    });
  }

  function playBlob(blob, controller) {
    return new Promise((resolve, reject) => {
      if (controller.signal.aborted) {
        resolve();
        return;
      }

      audioUrl = URL.createObjectURL(blob);
      audio = new Audio(audioUrl);

      const cleanup = () => {
        audio?.removeEventListener("ended", finish);
        audio?.removeEventListener("error", fail);
        controller.signal.removeEventListener("abort", abort);
      };
      const finish = () => {
        cleanup();
        releaseAudio();
        resolve();
      };
      const fail = () => {
        cleanup();
        releaseAudio();
        reject(new Error("Audio playback failed"));
      };
      const abort = () => {
        cleanup();
        releaseAudio();
        resolve();
      };

      audio.addEventListener("ended", finish, { once: true });
      audio.addEventListener("error", fail, { once: true });
      controller.signal.addEventListener("abort", abort, { once: true });
      audio.play().catch((error) => {
        cleanup();
        releaseAudio();
        reject(error);
      });
    });
  }

  async function fetchAndPlay(text, controller, delayMs = 0) {
    const response = await fetch("/api/tts", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ text }),
      signal: controller.signal,
    });
    if (!response.ok) {
      console.warn("TTS request failed", await response.text());
      return;
    }

    const blob = await response.blob();
    if (controller.signal.aborted) return;

    if (delayMs > 0) {
      await waitForPause(delayMs, controller);
      if (controller.signal.aborted) return;
    }

    await playBlob(blob, controller);
  }

  async function speak() {
    if (audio || abortController) {
      stop();
      return;
    }

    const items = getNotesPlaybackItems(getCurrentSlide());
    if (!items.length) return;

    const controller = new AbortController();
    abortController = controller;
    let applyInitialDelay = true;

    try {
      for (const item of items) {
        if (controller.signal.aborted) return;

        if (item.type === "pause") {
          await waitForPause(item.seconds * 1000, controller);
          continue;
        }

        const delayMs = applyInitialDelay ? initialDelayMs : 0;
        applyInitialDelay = false;
        await fetchAndPlay(item.text, controller, delayMs);
      }
    } catch (error) {
      if (error.name !== "AbortError") {
        console.warn("TTS playback failed", error);
        releaseAudio();
      }
    } finally {
      if (abortController === controller) abortController = null;
      releaseAudio();
    }
  }

  async function preload() {
    if (preloadPromise) return preloadPromise;

    preloadPromise = fetch("/api/tts/preload", { method: "POST" })
      .then(async (response) => {
        const summary = await response.json().catch(() => ({}));
        if (!response.ok) console.warn("TTS preload failed", summary);
        else console.info("TTS preload complete", summary);
        return summary;
      })
      .catch((error) => {
        console.warn("TTS preload failed", error);
        return null;
      })
      .finally(() => {
        preloadPromise = null;
      });

    return preloadPromise;
  }

  return { preload, speak, stop };
}
