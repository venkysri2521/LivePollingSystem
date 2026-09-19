import { useState } from "react";

export default function ShareBar({ url }) {
  const [copied, setCopied] = useState(false);

  async function copy() {
    try {
      await navigator.clipboard.writeText(url);
    } catch {
      // Clipboard access can be blocked; the input is selectable as a fallback.
    }
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  }

  return (
    <div className="share">
      <input className="share__url" value={url} readOnly onFocus={(e) => e.target.select()} aria-label="Poll link" />
      <button type="button" className="btn btn--solid" onClick={copy}>
        {copied ? "Copied" : "Copy link"}
      </button>
    </div>
  );
}
