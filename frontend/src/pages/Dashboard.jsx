import { useEffect, useState } from "react";
import { Link } from "react-router-dom";
import { api } from "../api/client.js";
import { useAuth } from "../context/AuthContext.jsx";
import ShareBar from "../components/ShareBar.jsx";

const blankOptions = ["", ""];

export default function Dashboard() {
  const { user } = useAuth();
  const [polls, setPolls] = useState([]);
  const [loading, setLoading] = useState(true);
  const [question, setQuestion] = useState("");
  const [options, setOptions] = useState(blankOptions);
  const [allowMultiple, setAllowMultiple] = useState(false);
  const [hideUntilVote, setHideUntilVote] = useState(false);
  const [closeInHours, setCloseInHours] = useState(0);
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  const [justCreated, setJustCreated] = useState(null);

  useEffect(() => {
    api
      .myPolls()
      .then((data) => setPolls(data.polls || []))
      .catch((err) => setError(err.message))
      .finally(() => setLoading(false));
  }, []);

  function updateOption(index, value) {
    setOptions(options.map((o, i) => (i === index ? value : o)));
  }

  async function createPoll(e) {
    e.preventDefault();
    setBusy(true);
    setError("");
    try {
      const data = await api.createPoll({
        question,
        options,
        allowMultiple,
        hideUntilVote,
        closeInHours: Number(closeInHours) || 0,
      });
      setPolls([data.poll, ...polls]);
      setJustCreated({ poll: data.poll, shareUrl: data.shareUrl });
      setQuestion("");
      setOptions(blankOptions);
      setAllowMultiple(false);
      setHideUntilVote(false);
      setCloseInHours(0);
    } catch (err) {
      setError(err.message);
    } finally {
      setBusy(false);
    }
  }

  async function toggleStatus(poll) {
    const next = poll.status === "open" ? "closed" : "open";
    try {
      const data = await api.setStatus(poll.slug, next);
      setPolls(polls.map((p) => (p.slug === poll.slug ? data.poll : p)));
    } catch (err) {
      setError(err.message);
    }
  }

  async function remove(poll) {
    if (!window.confirm(`Delete "${poll.question}" and all of its votes?`)) return;
    try {
      await api.deletePoll(poll.slug);
      setPolls(polls.filter((p) => p.slug !== poll.slug));
    } catch (err) {
      setError(err.message);
    }
  }

  return (
    <div className="page">
      <h1 className="h1">Hello, {user?.name}</h1>

      <section className="card">
        <h2 className="h2">New poll</h2>
        <form className="form" onSubmit={createPoll} noValidate>
          {error && <p className="alert">{error}</p>}

          <label className="field">
            <span>Question</span>
            <input
              value={question}
              maxLength={200}
              placeholder="What should we name the release?"
              onChange={(e) => setQuestion(e.target.value)}
              required
            />
          </label>

          <div className="field">
            <span>Options</span>
            <div className="options">
              {options.map((value, index) => (
                <div className="options__row" key={index}>
                  <input
                    value={value}
                    maxLength={80}
                    placeholder={`Option ${index + 1}`}
                    onChange={(e) => updateOption(index, e.target.value)}
                  />
                  {options.length > 2 && (
                    <button
                      type="button"
                      className="btn btn--quiet"
                      onClick={() => setOptions(options.filter((_, i) => i !== index))}
                      aria-label={`Remove option ${index + 1}`}
                    >
                      Remove
                    </button>
                  )}
                </div>
              ))}
            </div>
            {options.length < 10 && (
              <button type="button" className="btn" onClick={() => setOptions([...options, ""])}>
                Add option
              </button>
            )}
          </div>

          <div className="settings">
            <label className="check">
              <input
                type="checkbox"
                checked={allowMultiple}
                onChange={(e) => setAllowMultiple(e.target.checked)}
              />
              <span>Let people pick more than one</span>
            </label>

            <label className="check">
              <input
                type="checkbox"
                checked={hideUntilVote}
                onChange={(e) => setHideUntilVote(e.target.checked)}
              />
              <span>Hide results until someone votes</span>
            </label>

            <label className="field field--inline">
              <span>Close automatically</span>
              <select value={closeInHours} onChange={(e) => setCloseInHours(e.target.value)}>
                <option value={0}>Never</option>
                <option value={1}>In 1 hour</option>
                <option value={6}>In 6 hours</option>
                <option value={24}>In 24 hours</option>
                <option value={168}>In a week</option>
              </select>
            </label>
          </div>

          <button className="btn btn--solid btn--lg" disabled={busy}>
            {busy ? "Creating…" : "Create poll"}
          </button>
        </form>

        {justCreated && (
          <div className="created">
            <p>Your poll is live. Share this link and open it yourself to watch the results.</p>
            <ShareBar url={justCreated.shareUrl} />
          </div>
        )}
      </section>

      <section>
        <h2 className="h2">Your polls</h2>
        {loading && <p className="muted">Loading…</p>}
        {!loading && polls.length === 0 && (
          <p className="muted">Nothing here yet. The poll you create above will show up in this list.</p>
        )}

        <ul className="polls">
          {polls.map((poll) => (
            <li className="polls__item" key={poll.slug}>
              <div>
                <Link className="polls__q" to={`/p/${poll.slug}`}>
                  {poll.question}
                </Link>
                <p className="polls__meta">
                  {poll.totalVotes} {poll.totalVotes === 1 ? "vote" : "votes"} ·{" "}
                  {poll.status === "open" ? "taking votes" : "closed"}
                </p>
              </div>
              <div className="polls__actions">
                <button type="button" className="btn" onClick={() => toggleStatus(poll)}>
                  {poll.status === "open" ? "Close voting" : "Reopen"}
                </button>
                <button type="button" className="btn btn--danger" onClick={() => remove(poll)}>
                  Delete
                </button>
              </div>
            </li>
          ))}
        </ul>
      </section>
    </div>
  );
}
