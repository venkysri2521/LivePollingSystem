import { Link } from "react-router-dom";

export default function NotFound() {
  return (
    <div className="page page--center">
      <h1 className="h1">This page isn't here</h1>
      <p className="muted">The link may be mistyped, or the poll was deleted by its creator.</p>
      <Link to="/" className="btn btn--solid">
        Back to the start
      </Link>
    </div>
  );
}
