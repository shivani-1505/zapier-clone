import axios from 'axios';

// Create API client with default configuration
const api = axios.create({
  baseURL: process.env.REACT_APP_API_URL || 'http://localhost:8080',
  headers: {
    'Content-Type': 'application/json',
  },
});

// Add request interceptor for adding auth token
api.interceptors.request.use(
  (config) => {
    const token = localStorage.getItem('token');
    if (token) {
      config.headers.Authorization = `Bearer ${token}`;
    }
    return config;
  },
  (error) => Promise.reject(error)
);

// Add response interceptor for handling common errors
api.interceptors.response.use(
  (response) => response,
  async (error) => {
    const originalRequest = error.config;
    
    // Handle token expiration
    if (error.response && error.response.status === 401 && !originalRequest._retry) {
      originalRequest._retry = true;
      
      try {
        // Try to refresh the token
        const refreshToken = localStorage.getItem('refreshToken');
        if (refreshToken) {
          const res = await axios.post('/api/v1/auth/refresh', { token: refreshToken });
          
          if (res.data.token) {
            localStorage.setItem('token', res.data.token);
            api.defaults.headers.common['Authorization'] = `Bearer ${res.data.token}`;
            return api(originalRequest);
          }
        }
      } catch (err) {
        // If refresh fails, redirect to login
        console.error('Token refresh failed:', err);
        localStorage.removeItem('token');
        localStorage.removeItem('refreshToken');
        window.location.href = '/login';
      }
    }
    
    // Handle server errors
    if (error.response && error.response.status >= 500) {
      console.error('Server error:', error.response.data);
    }
    
    return Promise.reject(error);
  }
);

// General API functions

/**
 * Make a GET request
 * @param {string} url - API endpoint
 * @param {Object} params - URL parameters
 * @returns {Promise} - API response
 */
const get = async (url, params = {}) => {
  try {
    const response = await api.get(url, { params });
    return response.data;
  } catch (error) {
    handleApiError(error);
    throw error;
  }
};

/**
 * Make a POST request
 * @param {string} url - API endpoint
 * @param {Object} data - Request body
 * @returns {Promise} - API response
 */
const post = async (url, data = {}) => {
  try {
    const response = await api.post(url, data);
    return response.data;
  } catch (error) {
    handleApiError(error);
    throw error;
  }
};

/**
 * Make a PUT request
 * @param {string} url - API endpoint
 * @param {Object} data - Request body
 * @returns {Promise} - API response
 */
const put = async (url, data = {}) => {
  try {
    const response = await api.put(url, data);
    return response.data;
  } catch (error) {
    handleApiError(error);
    throw error;
  }
};

/**
 * Make a DELETE request
 * @param {string} url - API endpoint
 * @returns {Promise} - API response
 */
const del = async (url) => {
  try {
    const response = await api.delete(url);
    return response.data;
  } catch (error) {
    handleApiError(error);
    throw error;
  }
};

/**
 * Handle API errors
 * @param {Error} error - Error object
 */
const handleApiError = (error) => {
  if (error.response) {
    // The request was made and the server responded with a status code
    // that falls out of the range of 2xx
    console.error('API Error Response:', error.response.data);
    console.error('Status:', error.response.status);
    console.error('Headers:', error.response.headers);
  } else if (error.request) {
    // The request was made but no response was received
    console.error('API Error Request:', error.request);
  } else {
    // Something happened in setting up the request that triggered an Error
    console.error('API Error Message:', error.message);
  }
};

// Export the API client and utility functions
export default {
  client: api,
  get,
  post,
  put,
  delete: del,
};