import api from './api';

/**
 * List all connections
 * @returns {Promise<Object>} List of connections
 */
export const listConnections = async () => {
  try {
    return await api.get('/api/v1/connections');
  } catch (error) {
    console.error('Failed to list connections:', error);
    throw error;
  }
};

/**
 * Get a specific connection by ID
 * @param {string} id - Connection ID
 * @returns {Promise<Object>} Connection details
 */
export const getConnection = async (id) => {
  try {
    return await api.get(`/api/v1/connections/${id}`);
  } catch (error) {
    console.error(`Failed to get connection ${id}:`, error);
    throw error;
  }
};

/**
 * Create a new connection
 * @param {Object} connection - Connection data
 * @returns {Promise<Object>} Created connection
 */
export const createConnection = async (connection) => {
  try {
    return await api.post('/api/v1/connections', connection);
  } catch (error) {
    console.error('Failed to create connection:', error);
    throw error;
  }
};

/**
 * Update an existing connection
 * @param {string} id - Connection ID
 * @param {Object} connection - Connection data to update
 * @returns {Promise<Object>} Update result
 */
export const updateConnection = async (id, connection) => {
  try {
    return await api.put(`/api/v1/connections/${id}`, connection);
  } catch (error) {
    console.error(`Failed to update connection ${id}:`, error);
    throw error;
  }
};

/**
 * Delete a connection
 * @param {string} id - Connection ID
 * @returns {Promise<Object>} Deletion result
 */
export const deleteConnection = async (id) => {
  try {
    return await api.delete(`/api/v1/connections/${id}`);
  } catch (error) {
    console.error(`Failed to delete connection ${id}:`, error);
    throw error;
  }
};

/**
 * Test a connection
 * @param {string} id - Connection ID
 * @returns {Promise<Object>} Test result
 */
export const testConnection = async (id) => {
  try {
    return await api.post(`/api/v1/connections/${id}/test`);
  } catch (error) {
    console.error(`Failed to test connection ${id}:`, error);
    throw error;
  }
};

/**
 * Get OAuth URL for a connection
 * @param {string} service - Service name
 * @returns {Promise<Object>} OAuth URL and state
 */
export const getOAuthURL = async (service) => {
  try {
    return await api.get(`/api/v1/connections/oauth/${service}/url`);
  } catch (error) {
    console.error(`Failed to get OAuth URL for ${service}:`, error);
    throw error;
  }
};

/**
 * Complete OAuth flow
 * @param {string} service - Service name
 * @param {string} code - Authorization code
 * @param {string} state - State token
 * @returns {Promise<Object>} Connection details
 */
export const completeOAuth = async (service, code, state) => {
  try {
    return await api.post(`/api/v1/auth/oauth/callback/${service}`, { code, state });
  } catch (error) {
    console.error(`Failed to complete OAuth for ${service}:`, error);
    throw error;
  }
};

/**
 * List connections for a specific service
 * @param {string} service - Service name
 * @returns {Promise<Object>} List of connections for the service
 */
export const listConnectionsByService = async (service) => {
  try {
    return await api.get(`/api/v1/connections?service=${service}`);
  } catch (error) {
    console.error(`Failed to list connections for ${service}:`, error);
    throw error;
  }
};

/**
 * Get connection status
 * @param {string} id - Connection ID
 * @returns {Promise<Object>} Connection status
 */
export const getConnectionStatus = async (id) => {
  try {
    const response = await api.get(`/api/v1/connections/${id}`);
    return { 
      status: response.status,
      lastUsed: response.last_used_at
    };
  } catch (error) {
    console.error(`Failed to get status for connection ${id}:`, error);
    throw error;
  }
};

/**
 * Create API key connection
 * @param {string} service - Service name
 * @param {string} name - Connection name
 * @param {Object} credentials - API credentials
 * @returns {Promise<Object>} Created connection
 */
export const createAPIKeyConnection = async (service, name, credentials) => {
  try {
    return await api.post('/api/v1/connections', {
      name,
      service,
      auth_type: 'api_key',
      auth_data: credentials
    });
  } catch (error) {
    console.error(`Failed to create API key connection for ${service}:`, error);
    throw error;
  }
};

/**
 * Start OAuth connection
 * @param {string} service - Service name
 * @param {string} name - Connection name
 * @returns {Promise<Object>} Created connection and OAuth URL
 */
export const startOAuthConnection = async (service, name) => {
  try {
    // First, create a pending connection
    const connectionResponse = await api.post('/api/v1/connections', {
      name,
      service,
      auth_type: 'oauth',
      auth_data: {}
    });
    
    // Then, get the OAuth URL
    const oauthResponse = await getOAuthURL(service);
    
    return {
      connection: connectionResponse,
      oauthUrl: oauthResponse.url,
      state: oauthResponse.state
    };
  } catch (error) {
    console.error(`Failed to start OAuth connection for ${service}:`, error);
    throw error;
  }
};