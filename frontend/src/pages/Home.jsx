import { Link } from "react-router-dom";
import { useAuth } from "../context/AuthContext.jsx";

// The hero is a working demonstration of the product's one idea: a result
// filling in. It runs once on load rather than looping, so the page settles.
const demo = [
  { text: "Ship it today", share: 0.52 },
  { text: "One more review pass", share: 0.31 },
  { text: "Rewrite the whole thing", share: 0.17 },
];

export default function Home() {
  const { user } = useAuth();

  return (
    <div className="page">
      <section className="hero">
        <div>
          <h1 className="hero__title">
            Ask a room something, and watch it answer.
          </h1>
          <p className="hero__sub">
            Make a poll, send the link, and everyone watching sees each vote land the
            moment it happens. No refresh, no waiting for the count to catch up.
          </p>
          <div className="hero__actions">
            <Link to={user ? "/dashboard" : "/signup"} className="btn btn--solid btn--lg">
              {user ? "Go to my polls" : "Start a poll"}
            </Link>
            {!user && (
              <Link to="/login" className="btn btn--lg">
                I have an account
              </Link>
            )}
          </div>
        </div>

        <div className="demo" aria-hidden="true">
          <p className="demo__question">Friday deploy?</p>
          {demo.map((d, i) => (
            <div className="row row--demo" key={d.text} style={{ "--delay": `${0.25 + i * 0.12}s` }}>
              <span className="row__fill row__fill--demo" style={{ "--to": d.share }} />
              <span className="row__label">{d.text}</span>
              <span className="row__figures">
                <span className="row__percent">{Math.round(d.share * 100)}%</span>
              </span>
            </div>
          ))}
          <p className="demo__foot">
            <span className="pulse" /> 214 people watching
          </p>
        </div>
      </section>
    </div>
  );
}
