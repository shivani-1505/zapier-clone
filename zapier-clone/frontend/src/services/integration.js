import api from './api';

/**
 * List all available integration services
 * @returns {Promise<Object>} Available services
 */
export const listIntegrations = async () => {
  try {
    return await api.get('/api/v1/integrations');
  } catch (error) {
    console.error('Failed to list integrations:', error);
    throw error;
  }
};

/**
 * Get integration details for a specific service
 * @param {string} service - Service identifier
 * @returns {Promise<Object>} Integration details including triggers and actions
 */
export const getIntegrationDetails = async (service) => {
  try {
    return await api.get(`/api/v1/integrations/${service}`);
  } catch (error) {
    console.error(`Failed to get ${service} integration details:`, error);
    throw error;
  }
};

/**
 * List available triggers for a service
 * @param {string} service - Service identifier
 * @returns {Promise<Object>} Available triggers
 */
export const listTriggers = async (service) => {
  try {
    return await api.get(`/api/v1/integrations/${service}/triggers`);
  } catch (error) {
    console.error(`Failed to list triggers for ${service}:`, error);
    throw error;
  }
};

/**
 * List available actions for a service
 * @param {string} service - Service identifier
 * @returns {Promise<Object>} Available actions
 */
export const listActions = async (service) => {
  try {
    return await api.get(`/api/v1/integrations/${service}/actions`);
  } catch (error) {
    console.error(`Failed to list actions for ${service}:`, error);
    throw error;
  }
};

/**
 * Get OAuth URL for a service
 * @param {string} service - Service identifier
 * @returns {Promise<Object>} OAuth URL and state token
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
 * Handle OAuth callback
 * @param {string} service - Service identifier
 * @param {string} code - Authorization code
 * @param {string} state - State token
 * @returns {Promise<Object>} Connection details
 */
export const handleOAuthCallback = async (service, code, state) => {
  try {
    return await api.post(`/api/v1/auth/oauth/callback/${service}`, { code, state });
  } catch (error) {
    console.error(`Failed to handle OAuth callback for ${service}:`, error);
    throw error;
  }
};

/**
 * List available transformers
 * @returns {Promise<Object>} Available data transformers
 */
export const listTransformers = async () => {
  try {
    return await api.get('/api/v1/transformers');
  } catch (error) {
    console.error('Failed to list transformers:', error);
    throw error;
  }
};

/**
 * Test a trigger with sample data
 * @param {string} service - Service identifier
 * @param {string} triggerId - Trigger identifier
 * @param {Object} config - Trigger configuration
 * @returns {Promise<Object>} Sample trigger output
 */
export const testTrigger = async (service, triggerId, config) => {
  try {
    return await api.post(`/api/v1/integrations/${service}/triggers/${triggerId}/test`, { config });
  } catch (error) {
    console.error(`Failed to test trigger ${triggerId}:`, error);
    throw error;
  }
};

/**
 * Test an action with sample data
 * @param {string} service - Service identifier
 * @param {string} actionId - Action identifier
 * @param {Object} config - Action configuration
 * @param {Object} inputData - Sample input data
 * @returns {Promise<Object>} Sample action output
 */
export const testAction = async (service, actionId, config, inputData) => {
  try {
    return await api.post(`/api/v1/integrations/${service}/actions/${actionId}/test`, { 
      config, 
      input_data: inputData 
    });
  } catch (error) {
    console.error(`Failed to test action ${actionId}:`, error);
    throw error;
  }
};

/**
 * Get field schema for a trigger or action
 * @param {string} service - Service identifier
 * @param {string} type - 'trigger' or 'action'
 * @param {string} id - Trigger or action identifier
 * @returns {Promise<Object>} Field schema
 */
export const getFieldSchema = async (service, type, id) => {
  try {
    return await api.get(`/api/v1/integrations/${service}/${type}s/${id}/schema`);
  } catch (error) {
    console.error(`Failed to get schema for ${type} ${id}:`, error);
    throw error;
  }
};