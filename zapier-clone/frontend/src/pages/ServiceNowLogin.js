import React, { useState } from 'react';
import { Shield } from 'lucide-react';

const ServiceNowLogin = () => {
  const [username, setUsername] = useState('');
  const [password, setPassword] = useState('');
  const [error, setError] = useState('');

  const handleSubmit = (e) => {
    e.preventDefault();
    
    // Basic validation
    if (!username.trim() || !password.trim()) {
      setError('Please enter both username and password');
      return;
    }

    // Mock authentication 
    // In a real app, this would be a call to an authentication service
    if (username.trim() && password.trim()) {
      // Redirect to ServiceNow dashboard
      window.location.href = 'http://localhost:3000/';
    }
  };

  const styles = {
    container: {
      display: 'flex',
      flexDirection: 'column',
      minHeight: '100vh',
      backgroundColor: '#f5f5f5',
      alignItems: 'center',
      justifyContent: 'center',
      padding: '20px'
    },
    loginCard: {
      backgroundColor: 'white',
      borderRadius: '12px',
      boxShadow: '0 10px 25px rgba(0,0,0,0.1)',
      width: '100%',
      maxWidth: '400px',
      padding: '32px',
      display: 'flex',
      flexDirection: 'column',
      alignItems: 'center'
    },
    header: {
      display: 'flex',
      flexDirection: 'column',
      alignItems: 'center',
      marginBottom: '24px'
    },
    iconContainer: {
      width: '64px',
      height: '64px',
      borderRadius: '50%',
      display: 'flex',
      alignItems: 'center',
      justifyContent: 'center',
      marginBottom: '16px',
      backgroundColor: 'rgba(0,84,166,0.1)'
    },
    title: {
      fontSize: '1.5rem',
      fontWeight: '600',
      color: '#333',
      marginBottom: '8px'
    },
    subtitle: {
      color: '#666',
      marginBottom: '24px',
      textAlign: 'center'
    },
    form: {
      width: '100%'
    },
    inputGroup: {
      marginBottom: '16px'
    },
    label: {
      display: 'block',
      marginBottom: '8px',
      color: '#555',
      fontWeight: '500'
    },
    input: {
      width: '100%',
      padding: '12px',
      border: '1px solid #ddd',
      borderRadius: '6px',
      fontSize: '1rem'
    },
    submitButton: {
      width: '100%',
      padding: '12px',
      backgroundColor: '#0054a6',
      color: 'white',
      border: 'none',
      borderRadius: '6px',
      fontSize: '1rem',
      fontWeight: '600',
      cursor: 'pointer',
      transition: 'background-color 0.3s ease'
    },
    errorMessage: {
      color: 'red',
      marginBottom: '16px',
      textAlign: 'center'
    },
    backLink: {
      marginTop: '16px',
      color: '#666',
      textDecoration: 'none',
      fontSize: '0.875rem'
    }
  };

  return (
    <div style={styles.container}>
      <div style={styles.loginCard}>
        <div style={styles.header}>
          <div style={styles.iconContainer}>
            <Shield size={40} color="#0054a6" />
          </div>
          <h2 style={styles.title}>ServiceNow GRC Login</h2>
          <p style={styles.subtitle}>Enter your credentials to access the ServiceNow platform</p>
        </div>

        <form onSubmit={handleSubmit} style={styles.form}>
          {error && <div style={styles.errorMessage}>{error}</div>}
          
          <div style={styles.inputGroup}>
            <label htmlFor="username" style={styles.label}>Username</label>
            <input 
              type="text" 
              id="username"
              value={username}
              onChange={(e) => {
                setUsername(e.target.value);
                setError('');
              }}
              style={styles.input}
              placeholder="Enter your username"
            />
          </div>

          <div style={styles.inputGroup}>
            <label htmlFor="password" style={styles.label}>Password</label>
            <input 
              type="password" 
              id="password"
              value={password}
              onChange={(e) => {
                setPassword(e.target.value);
                setError('');
              }}
              style={styles.input}
              placeholder="Enter your password"
            />
          </div>

          <button 
            type="submit" 
            style={styles.submitButton}
          >
            Log In
          </button>

          <div style={{ textAlign: 'center' }}>
            <a 
              href="/" 
              style={styles.backLink}
            >
              ← Back to Platform Selection
            </a>
          </div>
        </form>
      </div>
    </div>
  );
};

export default ServiceNowLogin;