import { useEffect, useMemo, useState } from "react";
import { useParams } from "react-router-dom";
import { api } from "../api/client.js";
import { useLiveResults } from "../hooks/useLiveResults.js";
import ResultRow from "../components/ResultRow.jsx";
import ShareBar from "../components/ShareBar.jsx";

export default function PollPage() {
  const { slug } = useParams();
  const [poll, setPoll] = useState(null);
  const [loadError, setLoadError] = useState("");
  const [hasVoted, setHasVoted] = useState(false);
  const [isOwner, setIsOwner] = useState(false);
  const [shareUrl, setShareUrl] = useState("");
  const [picked, setPicked] = useState([]);
  const [voteError, setVoteError] = useState("");
  const [submitting, setSubmitting] = useState(false);

  const live = useLiveResults(poll ? slug : null, null);

  useEffect(() => {
    let active = true;
    api
      .getPoll(slug)
      .then((data) => {
        if (!active) return;
        setPoll(data.poll);
        setHasVoted(data.hasVoted);
        setIsOwner(data.isOwner);
        setShareUrl(data.shareUrl);
        live.setCounts(data.counts || {});
        live.setTotal(data.total || 0);
      })
      .catch((err) => active && setLoadError(err.message));
    return () => {
      active = false;
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [slug]);

  const open = useMemo(() => {
    if (!poll || live.closed) return false;
    if (poll.status !== "open") return false;
    if (poll.closesAt && new Date(poll.closesAt) < new Date()) return false;
    return true;
  }, [poll, live.closed]);

  // Results stay hidden until you have voted only if the creator asked for it;
  // otherwise seeing the board is the whole point of opening the link.
  const revealed = hasVoted || !open || !poll?.hideUntilVote;

  const leaderId = useMemo(() => {
    const entries = Object.entries(live.counts);
    if (!entries.length) return null;
    const [id, value] = entries.reduce((a, b) => (b[1] > a[1] ? b : a));
    return value > 0 ? id : null;
  }, [live.counts]);

  function toggle(optionId) {
    if (poll.allowMultiple) {
      setPicked(picked.includes(optionId) ? picked.filter((id) => id !== optionId) : [...picked, optionId]);
    } else {
      setPicked([optionId]);
    }
  }

  async function submit() {
    setSubmitting(true);
    setVoteError("");
    try {
      const data = await api.vote(slug, picked);
      live.setCounts(data.counts);
      live.setTotal(data.total);
      setHasVoted(true);
    } catch (err) {
      // 409 means the server already has a vote from this person, which is a
      // real answer rather than a failure, so the board is revealed anyway.
      if (err.status === 409) setHasVoted(true);
      setVoteError(err.message);
    } finally {
      setSubmitting(false);
    }
  }

  async function closeVoting() {
    try {
      const data = await api.setStatus(slug, poll.status === "open" ? "closed" : "open");
      setPoll(data.poll);
    } catch (err) {
      setVoteError(err.message);
    }
  }

  if (loadError) {
    return (
      <div className="page page--center">
        <h1 className="h1">{loadError}</h1>
        <p className="muted">Double-check the link with whoever shared it.</p>
      </div>
    );
  }
  if (!poll) return <div className="page page--center muted">Loading the poll…</div>;

  return (
    <div className="page page--poll">
      <div className="pollHead">
        <p className="pollHead__by">Asked by {poll.ownerName}</p>
        <h1 className="pollHead__q">{poll.question}</h1>
        <div className="pollHead__state">
          <span className={`tag tag--${open ? "live" : "closed"}`}>
            {open ? (
              <>
                <span className="pulse" /> {live.status === "offline" ? "Reconnecting" : "Live"}
              </>
            ) : (
              "Voting closed"
            )}
          </span>
          <span className="tally">
            {live.total} {live.total === 1 ? "vote" : "votes"}
          </span>
          {live.viewers > 1 && <span className="tally">{live.viewers} watching</span>}
        </div>
      </div>

      <div className="board">
        {poll.options.map((option) => {
          const count = live.counts[option.id] ?? 0;
          return (
            <ResultRow
              key={option.id}
              option={option}
              count={count}
              total={live.total}
              share={live.total > 0 ? count / live.total : 0}
              selectable={open && !hasVoted}
              selected={picked.includes(option.id)}
              onSelect={toggle}
              leading={option.id === leaderId}
              flashing={live.changed.includes(option.id)}
              revealed={revealed}
            />
          );
        })}
      </div>

      {voteError && <p className="alert">{voteError}</p>}

      {open && !hasVoted && (
        <div className="voteBar">
          <p className="muted">
            {poll.allowMultiple ? "Pick as many as you like." : "Pick one."}
          </p>
          <button className="btn btn--solid btn--lg" onClick={submit} disabled={!picked.length || submitting}>
            {submitting ? "Sending…" : "Submit vote"}
          </button>
        </div>
      )}

      {hasVoted && open && <p className="muted">Your vote is counted. The numbers above keep moving on their own.</p>}
      {!open && <p className="muted">This poll is closed. These are the final numbers.</p>}

      <section className="pollFoot">
        <h2 className="h2">Share this poll</h2>
        <ShareBar url={shareUrl || window.location.href} />
        {isOwner && (
          <button type="button" className="btn" onClick={closeVoting}>
            {poll.status === "open" ? "Close voting" : "Reopen voting"}
          </button>
        )}
      </section>
    </div>
  );
}
