import { useEffect, useRef, useState } from "react";
import { streamUrl } from "../api/client.js";

/**
 * Holds one EventSource per poll and keeps the counts in sync with it.
 *
 * EventSource reconnects by itself, so the job here is narrower than it looks:
 * track connection state for the UI, remember which option numbers changed so
 * the row can flash, and tear the connection down on unmount.
 */
export function useLiveResults(slug, initial) {
  const [counts, setCounts] = useState(initial?.counts ?? {});
  const [total, setTotal] = useState(initial?.total ?? 0);
  const [viewers, setViewers] = useState(0);
  const [status, setStatus] = useState("connecting"); // connecting | live | offline
  const [closed, setClosed] = useState(false);
  const [changed, setChanged] = useState([]);
  const previous = useRef(initial?.counts ?? {});
  const seenFirst = useRef(false);

  useEffect(() => {
    if (!slug) return undefined;

    const source = new EventSource(streamUrl(slug), { withCredentials: true });

    source.onopen = () => setStatus("live");

    source.onmessage = (event) => {
      let snap;
      try {
        snap = JSON.parse(event.data);
      } catch {
        return;
      }
      setStatus("live");

      const next = snap.counts || {};
      const moved = Object.keys(next).filter((id) => next[id] !== previous.current[id]);
      previous.current = next;

      setCounts(next);
      setTotal(snap.total ?? 0);
      if (typeof snap.viewers === "number") setViewers(snap.viewers);
      if (snap.type === "closed") setClosed(true);
      // The first snapshot is just the current state, not news, so it does not
      // flash - only movement that happens while you are watching does.
      if (moved.length && seenFirst.current) setChanged(moved);
      seenFirst.current = true;
    };

    // The browser retries on its own; showing "reconnecting" is more honest
    // than pretending the numbers on screen are still current.
    source.onerror = () => setStatus("offline");

    return () => source.close();
  }, [slug]);

  // Clear the highlight after the flash so a later vote can re-trigger it.
  useEffect(() => {
    if (!changed.length) return undefined;
    const timer = setTimeout(() => setChanged([]), 900);
    return () => clearTimeout(timer);
  }, [changed]);

  return { counts, total, viewers, status, closed, changed, setCounts, setTotal };
}
