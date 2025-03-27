import React, { createContext, useState, useContext, useEffect } from 'react';
import { useNavigate } from 'react-router-dom';
import api from '../services/api';

// Create the auth context
const AuthContext = createContext();

// Auth provider component
export const AuthProvider = ({ children }) => {
  const [isAuthenticated, setIsAuthenticated] = useState(false);
  const [user, setUser] = useState(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(null);
  const navigate = useNavigate();

  // Initialize auth state on component mount
  useEffect(() => {
    const initAuth = async () => {
      try {
        // Check for token in localStorage
        const token = localStorage.getItem('token');
        
        if (!token) {
          setLoading(false);
          return;
        }
        
        // Set default auth header
        api.defaults.headers.common['Authorization'] = `Bearer ${token}`;
        
        // Fetch user profile
        const response = await api.get('/api/v1/user/profile');
        
        if (response.data && response.data.email) {
          setUser(response.data);
          setIsAuthenticated(true);
        } else {
          // Clear invalid token
          localStorage.removeItem('token');
          delete api.defaults.headers.common['Authorization'];
        }
      } catch (err) {
        console.error('Auth initialization error:', err);
        // Clear invalid token
        localStorage.removeItem('token');
        delete api.defaults.headers.common['Authorization'];
        setError('Session expired. Please login again.');
      } finally {
        setLoading(false);
      }
    };

    initAuth();
  }, []);

  // Login function
  const login = async (email, password) => {
    try {
      setError(null);
      setLoading(true);
      
      const response = await api.post('/api/v1/auth/login', { email, password });
      
      if (response.data && response.data.token) {
        localStorage.setItem('token', response.data.token);
        api.defaults.headers.common['Authorization'] = `Bearer ${response.data.token}`;
        
        // Fetch user profile
        const userResponse = await api.get('/api/v1/user/profile');
        setUser(userResponse.data);
        setIsAuthenticated(true);
        
        return true;
      } else {
        setError('Login failed. Please check your credentials.');
        return false;
      }
    } catch (err) {
      console.error('Login error:', err);
      setError(err.response?.data?.error || 'Login failed. Please try again.');
      return false;
    } finally {
      setLoading(false);
    }
  };

  // Register function
  const register = async (username, email, password) => {
    try {
      setError(null);
      setLoading(true);
      
      const response = await api.post('/api/v1/auth/register', { 
        username, 
        email, 
        password 
      });
      
      if (response.data && response.data.token) {
        localStorage.setItem('token', response.data.token);
        api.defaults.headers.common['Authorization'] = `Bearer ${response.data.token}`;
        
        // Fetch user profile
        const userResponse = await api.get('/api/v1/user/profile');
        setUser(userResponse.data);
        setIsAuthenticated(true);
        
        return true;
      } else {
        setError('Registration failed. Please try again.');
        return false;
      }
    } catch (err) {
      console.error('Registration error:', err);
      setError(err.response?.data?.error || 'Registration failed. Please try again.');
      return false;
    } finally {
      setLoading(false);
    }
  };

  // Logout function
  const logout = () => {
    localStorage.removeItem('token');
    delete api.defaults.headers.common['Authorization'];
    setUser(null);
    setIsAuthenticated(false);
    navigate('/login');
  };

  // Update user profile
  const updateProfile = async (userData) => {
    try {
      setError(null);
      setLoading(true);
      
      const response = await api.put('/api/v1/user/profile', userData);
      
      if (response.data) {
        setUser(prev => ({ ...prev, ...response.data }));
        return true;
      } else {
        setError('Failed to update profile. Please try again.');
        return false;
      }
    } catch (err) {
      console.error('Profile update error:', err);
      setError(err.response?.data?.error || 'Failed to update profile. Please try again.');
      return false;
    } finally {
      setLoading(false);
    }
  };

  // Change password
  const changePassword = async (currentPassword, newPassword) => {
    try {
      setError(null);
      setLoading(true);
      
      const response = await api.put('/api/v1/user/password', { 
        current_password: currentPassword, 
        new_password: newPassword 
      });
      
      if (response.data && response.data.success) {
        return true;
      } else {
        setError('Failed to change password. Please try again.');
        return false;
      }
    } catch (err) {
      console.error('Password change error:', err);
      setError(err.response?.data?.error || 'Failed to change password. Please try again.');
      return false;
    } finally {
      setLoading(false);
    }
  };

  // Refresh token function
  const refreshToken = async () => {
    try {
      const currentToken = localStorage.getItem('token');
      
      if (!currentToken) {
        logout();
        return false;
      }
      
      const response = await api.post('/api/v1/auth/refresh', {
        token: currentToken
      });
      
      if (response.data && response.data.token) {
        localStorage.setItem('token', response.data.token);
        api.defaults.headers.common['Authorization'] = `Bearer ${response.data.token}`;
        return true;
      } else {
        logout();
        return false;
      }
    } catch (err) {
      console.error('Token refresh error:', err);
      logout();
      return false;
    }
  };

  // Context value
  const value = {
    isAuthenticated,
    user,
    loading,
    error,
    login,
    register,
    logout,
    updateProfile,
    changePassword,
    refreshToken
  };

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>;
};

// Custom hook to use the auth context
export const useAuth = () => {
  const context = useContext(AuthContext);
  if (!context) {
    throw new Error('useAuth must be used within an AuthProvider');
  }
  return context;
};

// Export auth context
export default AuthContext;