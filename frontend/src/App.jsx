import { Routes, Route } from "react-router-dom";
import Navbar from "./components/Navbar.jsx";
import ProtectedRoute from "./components/ProtectedRoute.jsx";
import Home from "./pages/Home.jsx";
import Login from "./pages/Login.jsx";
import Signup from "./pages/Signup.jsx";
import Dashboard from "./pages/Dashboard.jsx";
import PollPage from "./pages/PollPage.jsx";
import NotFound from "./pages/NotFound.jsx";

export default function App() {
  return (
    <div className="shell">
      <Navbar />
      <main>
        <Routes>
          <Route path="/" element={<Home />} />
          <Route path="/login" element={<Login />} />
          <Route path="/signup" element={<Signup />} />
          <Route
            path="/dashboard"
            element={
              <ProtectedRoute>
                <Dashboard />
              </ProtectedRoute>
            }
          />
          <Route path="/p/:slug" element={<PollPage />} />
          <Route path="*" element={<NotFound />} />
        </Routes>
      </main>
      <footer className="foot">
        Built with React, Go, MongoDB and Redis.
      </footer>
    </div>
  );
}
