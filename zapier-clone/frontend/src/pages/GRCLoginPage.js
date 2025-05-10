import React, { useState } from 'react';
import { Bell, Shield, AlertTriangle, ChevronRight } from 'lucide-react';

const GRCLoginPage = () => {
  const [hoveredCard, setHoveredCard] = useState(null);
  
const platforms = [
  {
    id: 'servicenow',
    name: 'ServiceNow GRC',
    description: 'Manage risks, compliance tasks, and monitor regulatory changes in your ServiceNow instance.',
    loginUrl: '/login/servicenow', // Note the leading slash
    dashboardUrl: 'http://localhost:3000/dashboard',
    color: '#0054a6',
    icon: (size) => <Shield size={size} />,
    features: ['Risk Management', 'Compliance Tasks', 'Regulatory Changes']
  },
  {
    id: 'slack',
    name: 'Slack Workspace',
    description: 'Connect to our mock Slack server for team communication and collaboration.',
    loginUrl: '/login/slack', // Note the leading slash
    dashboardUrl: 'http://localhost:3002/channels',
    color: '#4a154b',
    icon: (size) => <Bell size={size} />,
    features: ['Team Communication', 'Channel Messaging', 'File Sharing']
  },
  {
    id: 'jira',
    name: 'Jira Risk Tickets',
    description: 'Track and manage risk tickets created from ServiceNow alerts in your Jira project board.',
    loginUrl: '/login/jira', // Note the leading slash
    dashboardUrl: 'http://localhost:3001/board',
    color: '#0052cc',
    icon: (size) => <AlertTriangle size={size} />,
    features: ['Ticket Tracking', 'Risk Resolution', 'Workflow Management']
  }
];

  const styles = {
    container: {
      display: 'flex',
      flexDirection: 'column',
      minHeight: '100vh',
      backgroundColor: '#f5f5f5'
    },
    header: {
      background: 'linear-gradient(to right, #172b4d, #2563eb)',
      color: 'white',
      boxShadow: '0 4px 6px rgba(0, 0, 0, 0.1)'
    },
    headerContainer: {
      maxWidth: '1200px',
      margin: '0 auto',
      padding: '16px 24px',
    },
    headerContent: {
      display: 'flex',
      alignItems: 'center',
      justifyContent: 'space-between'
    },
    headerTitle: {
      display: 'flex',
      alignItems: 'center',
      gap: '12px'
    },
    headerInfo: {
      display: 'flex',
      alignItems: 'center',
      gap: '16px'
    },
    divider: {
      width: '1px',
      height: '24px',
      backgroundColor: 'rgba(255, 255, 255, 0.3)'
    },
    main: {
      flex: 1,
      display: 'flex',
      alignItems: 'center',
      justifyContent: 'center',
      padding: '24px'
    },
    content: {
      width: '100%',
      maxWidth: '1200px'
    },
    titleSection: {
      textAlign: 'center',
      marginBottom: '40px'
    },
    title: {
      fontSize: '1.875rem',
      fontWeight: 'bold',
      color: '#333'
    },
    subtitle: {
      marginTop: '12px',
      color: '#666',
      maxWidth: '600px',
      margin: '12px auto 0'
    },
    cardGrid: {
      display: 'grid',
      gridTemplateColumns: 'repeat(auto-fill, minmax(300px, 1fr))',
      gap: '32px'
    },
    card: {
      backgroundColor: 'white',
      borderRadius: '12px',
      overflow: 'hidden',
      boxShadow: '0 4px 8px rgba(0, 0, 0, 0.05)',
      transition: 'all 0.3s ease'
    },
    cardHeader: {
      height: '12px',
      width: '100%'
    },
    cardBody: {
      padding: '24px'
    },
    cardTitleSection: {
      display: 'flex',
      alignItems: 'center',
      marginBottom: '16px'
    },
    iconContainer: {
      width: '48px',
      height: '48px',
      borderRadius: '50%',
      display: 'flex',
      alignItems: 'center',
      justifyContent: 'center',
      marginRight: '16px'
    },
    cardTitle: {
      fontSize: '1.25rem',
      fontWeight: '600'
    },
    cardDescription: {
      color: '#666',
      marginBottom: '24px',
      height: '64px'
    },
    featuresTitle: {
      fontSize: '0.875rem',
      fontWeight: '600',
      color: '#888',
      textTransform: 'uppercase',
      letterSpacing: '0.05em',
      marginBottom: '8px'
    },
    featuresList: {
      listStyle: 'none',
      padding: 0,
      margin: 0,
      marginBottom: '24px'
    },
    featureItem: {
      display: 'flex',
      alignItems: 'center',
      color: '#555',
      marginBottom: '4px'
    },
    featureIcon: {
      marginRight: '4px'
    },
    loginButtonContainer: {
      display: 'flex',
      justifyContent: 'center',
      width: '100%'
    },
    loginButton: {
      display: 'inline-flex',
      alignItems: 'center',
      justifyContent: 'center',
      width: '80%', // Reduced width to make button less stretched
      padding: '12px 16px',
      borderRadius: '6px',
      fontWeight: '500',
      color: 'white',
      textDecoration: 'none',
      transition: 'background-color 0.3s ease'
    },
    footer: {
      backgroundColor: 'white',
      borderTop: '1px solid #eaeaea',
      padding: '16px 0'
    },
    footerContainer: {
      maxWidth: '1200px',
      margin: '0 auto',
      padding: '0 24px',
      display: 'flex',
      flexDirection: 'column',
      alignItems: 'center'
    },
    footerText: {
      color: '#666',
      fontSize: '0.875rem'
    }
  };

  return (
    <div style={styles.container}>
      {/* Header */}
      <header style={styles.header}>
        <div style={styles.headerContainer}>
          <div style={styles.headerContent}>
            <div style={styles.headerTitle}>
              <Shield size={28} />
              <h1 style={{ fontSize: '1.5rem', fontWeight: 'bold' }}>GRC Integration Framework</h1>
            </div>
            <div style={styles.headerInfo}>
              <span style={{ fontSize: '0.875rem', opacity: 0.8 }}>Version 1.0.0</span>
              <div style={styles.divider}></div>
              <span style={{ fontSize: '0.875rem', opacity: 0.8 }}>Built with Go & React</span>
            </div>
          </div>
        </div>
      </header>

      {/* Main Content */}
      <main style={styles.main}>
        <div style={styles.content}>
          <div style={styles.titleSection}>
            <h2 style={styles.title}>Welcome to Your GRC Platform</h2>
            <p style={styles.subtitle}>
              Select a platform to log in and manage your governance, risk, and compliance workflow
            </p>
          </div>

          <div style={styles.cardGrid}>
            {platforms.map((platform) => (
              <div 
                key={platform.id}
                style={{
                  ...styles.card,
                  transform: hoveredCard === platform.id ? 'translateY(-8px)' : 'none',
                  boxShadow: hoveredCard === platform.id ? '0 10px 15px rgba(0, 0, 0, 0.1)' : styles.card.boxShadow
                }}
                onMouseEnter={() => setHoveredCard(platform.id)}
                onMouseLeave={() => setHoveredCard(null)}
              >
                <div 
                  style={{
                    ...styles.cardHeader,
                    backgroundColor: platform.color
                  }}
                ></div>
                <div style={styles.cardBody}>
                  <div style={styles.cardTitleSection}>
                    <div 
                      style={{
                        ...styles.iconContainer,
                        backgroundColor: `${platform.color}20`
                      }}
                    >
                      {platform.icon(24)}
                    </div>
                    <h3 style={styles.cardTitle}>{platform.name}</h3>
                  </div>

                  <p style={styles.cardDescription}>{platform.description}</p>

                  <div>
                    <h4 style={styles.featuresTitle}>Features</h4>
                    <ul style={styles.featuresList}>
                      {platform.features.map((feature, idx) => (
                        <li key={idx} style={styles.featureItem}>
                          <ChevronRight size={16} style={styles.featureIcon} />
                          <span>{feature}</span>
                        </li>
                      ))}
                    </ul>
                  </div>

                  <div style={styles.loginButtonContainer}>
                    <a 
                      href={platform.loginUrl}
                      style={{
                        ...styles.loginButton,
                        backgroundColor: platform.color,
                        boxShadow: hoveredCard === platform.id ? `0 4px 12px ${platform.color}40` : 'none'
                      }}
                    >
                      Login to {platform.name.split(' ')[0]}
                    </a>
                  </div>
                </div>
              </div>
            ))}
          </div>
        </div>
      </main>

      {/* Footer */}
      <footer style={styles.footer}>
        <div style={styles.footerContainer}>
          <p style={styles.footerText}>&copy; 2025 GRC Integration Framework | All Rights Reserved</p>
        </div>
      </footer>
    </div>
  );
};

export default GRCLoginPage;