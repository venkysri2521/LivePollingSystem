import { useState } from "react";
import { Link, useNavigate } from "react-router-dom";
import { useAuth } from "../context/AuthContext.jsx";

export default function Signup() {
  const { register } = useAuth();
  const navigate = useNavigate();
  const [form, setForm] = useState({ name: "", email: "", password: "" });
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);

  async function submit(e) {
    e.preventDefault();
    setBusy(true);
    setError("");
    try {
      await register(form);
      navigate("/dashboard", { replace: true });
    } catch (err) {
      setError(err.message);
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="page page--narrow">
      <h1 className="h1">Create an account</h1>
      <p className="muted">You need one to make and manage polls. Voting never requires an account.</p>

      <form className="card form" onSubmit={submit} noValidate>
        {error && <p className="alert">{error}</p>}

        <label className="field">
          <span>Name</span>
          <input
            value={form.name}
            autoComplete="name"
            onChange={(e) => setForm({ ...form, name: e.target.value })}
            required
          />
        </label>

        <label className="field">
          <span>Email</span>
          <input
            type="email"
            autoComplete="email"
            value={form.email}
            onChange={(e) => setForm({ ...form, email: e.target.value })}
            required
          />
        </label>

        <label className="field">
          <span>Password</span>
          <input
            type="password"
            autoComplete="new-password"
            value={form.password}
            onChange={(e) => setForm({ ...form, password: e.target.value })}
            required
          />
          <small>At least 8 characters, with a letter and a number.</small>
        </label>

        <button className="btn btn--solid btn--lg" disabled={busy}>
          {busy ? "Creating…" : "Create account"}
        </button>
      </form>
      <p className="muted">
        Already signed up? <Link to="/login">Sign in</Link>
      </p>
    </div>
  );
}
