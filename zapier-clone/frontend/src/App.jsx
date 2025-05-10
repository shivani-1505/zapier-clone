import React from 'react';
import { BrowserRouter as Router, Routes, Route } from 'react-router-dom';
import GRCLoginPage from './pages/GRCLoginPage';
import ServiceNowLogin from './pages/ServiceNowLogin';
import SlackLogin from './pages/SlackLogin';
import JiraLogin from './pages/JiraLogin';

function App() {
  return (
    <Router>
      <Routes>
        <Route path="/" element={<GRCLoginPage />} />
        <Route path="/login/servicenow" element={<ServiceNowLogin />} />
        <Route path="/login/slack" element={<SlackLogin />} />
        <Route path="/login/jira" element={<JiraLogin />} />
      </Routes>
    </Router>
  );
}

export default App;