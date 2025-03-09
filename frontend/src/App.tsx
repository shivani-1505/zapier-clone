import React, { useState } from 'react';
import { BrowserRouter as Router, Routes, Route, Link } from 'react-router-dom';
import './App.css';

function App() {
  return (
    <Router>
      <div className="App">
        <header className="App-header">
          <h1>AuditCue Integration Manager</h1>
          <nav>
            <Link to="/">Home</Link> | 
            <Link to="/setup">Setup New Integration</Link>
          </nav>
        </header>
        
        <Routes>
          <Route path="/" element={<Home />} />
          <Route path="/setup" element={<SetupIntegration />} />
          <Route path="/success" element={<SuccessPage />} />
        </Routes>
      </div>
    </Router>
  );
}

function Home() {
  return (
    <div className="container">
      <h2>Welcome to AuditCue Integration Manager</h2>
      <p>This application allows you to create integrations between Slack and Gmail.</p>
      <p>Click "Setup New Integration" to get started.</p>
    </div>
  );
}

function SetupIntegration() {
  const [formData, setFormData] = useState({
    user_id: '',
    slack_bot_token: '',
    slack_team_id: ''
  });
  
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  
  const handleChange = (e: { target: { name: any; value: any; }; }) => {
    const { name, value } = e.target;
    setFormData(prev => ({
      ...prev,
      [name]: value
    }));
  };
  
  const handleSubmit = async (e: { preventDefault: () => void; }) => {
    e.preventDefault();
    setLoading(true);
    setError(null);
    
    try {
      console.log("Submitting to API...");
      
      // Use the full URL to your Railway deployment
      const apiUrl = 'https://integration-production-25ff.up.railway.app/api/auth/credentials';
      
      const response = await fetch(apiUrl, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json'
        },
        body: JSON.stringify(formData),
        // Important: Don't follow redirects automatically
        redirect: 'manual'
      });
      
      console.log("Response status:", response.status, response.type);
      
      // Check if response is a redirect
      if (response.type === 'opaqueredirect' || 
          response.status === 301 || 
          response.status === 302 || 
          response.status === 303) {
        console.log("Received redirect, navigating to /success");
        window.location.href = '/success';
        return;
      }
      
      if (!response.ok) {
        throw new Error(`Server responded with ${response.status}`);
      }
      
      try {
        const data = await response.json();
        console.log("Response data:", data);
        
        if (data.status === 'success') {
          console.log("Success! Redirecting to /success");
          window.location.href = '/success';
        } else if (data.auth_url) {
          window.location.href = data.auth_url;
        } else {
          window.location.href = '/success';
        }
      } catch (jsonError) {
        console.log("Couldn't parse JSON, but still received successful response");
        window.location.href = '/success';
      }
    } catch (err) {
      console.error("Fetch error:", err);
      if (err instanceof Error) {
        setError(err.message);
      } else {
        setError('An unknown error occurred');
      }
    } finally {
      setLoading(false);
    }
  };
  
  return (
    <div className="container">
      <h2>Setup New Integration</h2>
      
      {error && <div className="error-box">{error}</div>}
      
      <form onSubmit={handleSubmit}>
        <div className="form-group">
          <label htmlFor="user_id">User ID (any unique identifier):</label>
          <input
            type="text"
            id="user_id"
            name="user_id"
            value={formData.user_id}
            onChange={handleChange}
            required
          />
          <small>This ID will be used to associate your Slack workspace with this integration</small>
        </div>
        
        <div className="section-header">
          <h3>Slack Configuration</h3>
        </div>
        
        <div className="form-group">
          <label htmlFor="slack_bot_token">Slack Bot Token:</label>
          <input
            type="password"
            id="slack_bot_token"
            name="slack_bot_token"
            value={formData.slack_bot_token}
            onChange={handleChange}
            required
          />
          <small>Starting with xoxb- or xoxp-</small>
        </div>
        
        <div className="form-group">
          <label htmlFor="slack_team_id">Slack Team ID:</label>
          <input
            type="text"
            id="slack_team_id"
            name="slack_team_id"
            value={formData.slack_team_id}
            onChange={handleChange}
            required
          />
          <small>You can find your Slack Team ID in the Slack API dashboard</small>
        </div>
        
        <div className="info-box">
          <p><strong>Note:</strong> Email notifications will be sent from our system email: connectify.workflow@gmail.com</p>
        </div>
        
        <button type="submit" disabled={loading}>
          {loading ? 'Connecting...' : 'Connect Slack Workspace'}
        </button>
      </form>
    </div>
  );
}

function SuccessPage() {
  return (
    <div className="container success" style={{color: 'black'}}>
      <h2>✅ Integration Setup Complete!</h2>
      <p>Your Slack workspace has been successfully connected.</p>
      <p>Any new messages in your configured Slack channels will trigger email notifications to channel members.</p>
      
      <div className="next-steps">
        <h3>Next Steps:</h3>
        <ol>
          <li>Make sure your Slack Bot is invited to the channels you want to monitor</li>
          <li>Configure your Slack Event Subscriptions to point to your server URL at 
            <code>https://integration-production-25ff.up.railway.app/api/slack/events</code>
          </li>
          <li>Subscribe to the 'message.channels' event in the Slack API dashboard</li>
        </ol>
      </div>
      
      <Link to="/" className="button">Return to Home</Link>
    </div>
  );
}

export default App;