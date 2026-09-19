import { Link, useNavigate } from "react-router-dom";
import { useAuth } from "../context/AuthContext.jsx";

export default function Navbar() {
  const { user, signOut } = useAuth();
  const navigate = useNavigate();

  return (
    <header className="nav">
      <Link to="/" className="nav__mark">
        <span className="nav__markGlyph" aria-hidden="true" />
        Tally
      </Link>

      <nav className="nav__links">
        {user ? (
          <>
            <Link to="/dashboard">My polls</Link>
            <button
              type="button"
              className="btn btn--quiet"
              onClick={() => {
                signOut();
                navigate("/");
              }}
            >
              Sign out
            </button>
          </>
        ) : (
          <>
            <Link to="/login">Sign in</Link>
            <Link to="/signup" className="btn btn--solid btn--sm">
              Create a poll
            </Link>
          </>
        )}
      </nav>
    </header>
  );
}
