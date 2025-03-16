import api from './api';

/**
 * List all workflows
 * @returns {Promise<Object>} List of workflows
 */
export const listWorkflows = async () => {
  try {
    return await api.get('/api/v1/workflows');
  } catch (error) {
    console.error('Failed to list workflows:', error);
    throw error;
  }
};

/**
 * Get a specific workflow by ID
 * @param {string} id - Workflow ID
 * @returns {Promise<Object>} Workflow details
 */
export const getWorkflow = async (id) => {
  try {
    return await api.get(`/api/v1/workflows/${id}`);
  } catch (error) {
    console.error(`Failed to get workflow ${id}:`, error);
    throw error;
  }
};

/**
 * Create a new workflow
 * @param {Object} workflow - Workflow data
 * @returns {Promise<Object>} Created workflow
 */
export const createWorkflow = async (workflow) => {
  try {
    return await api.post('/api/v1/workflows', workflow);
  } catch (error) {
    console.error('Failed to create workflow:', error);
    throw error;
  }
};

/**
 * Update an existing workflow
 * @param {string} id - Workflow ID
 * @param {Object} workflow - Workflow data to update
 * @returns {Promise<Object>} Update result
 */
export const updateWorkflow = async (id, workflow) => {
  try {
    return await api.put(`/api/v1/workflows/${id}`, workflow);
  } catch (error) {
    console.error(`Failed to update workflow ${id}:`, error);
    throw error;
  }
};

/**
 * Delete a workflow
 * @param {string} id - Workflow ID
 * @returns {Promise<Object>} Deletion result
 */
export const deleteWorkflow = async (id) => {
  try {
    return await api.delete(`/api/v1/workflows/${id}`);
  } catch (error) {
    console.error(`Failed to delete workflow ${id}:`, error);
    throw error;
  }
};

/**
 * Activate a workflow
 * @param {string} id - Workflow ID
 * @returns {Promise<Object>} Activation result
 */
export const activateWorkflow = async (id) => {
  try {
    return await api.post(`/api/v1/workflows/${id}/activate`);
  } catch (error) {
    console.error(`Failed to activate workflow ${id}:`, error);
    throw error;
  }
};

/**
 * Deactivate a workflow
 * @param {string} id - Workflow ID
 * @returns {Promise<Object>} Deactivation result
 */
export const deactivateWorkflow = async (id) => {
  try {
    return await api.post(`/api/v1/workflows/${id}/deactivate`);
  } catch (error) {
    console.error(`Failed to deactivate workflow ${id}:`, error);
    throw error;
  }
};

/**
 * Test a workflow with sample trigger data
 * @param {string} id - Workflow ID
 * @param {Object} triggerData - Sample trigger data
 * @returns {Promise<Object>} Test result
 */
export const testWorkflow = async (id, triggerData) => {
  try {
    return await api.post(`/api/v1/workflows/${id}/test`, { trigger_data: triggerData });
  } catch (error) {
    console.error(`Failed to test workflow ${id}:`, error);
    throw error;
  }
};

/**
 * Add an action to a workflow
 * @param {string} workflowId - Workflow ID
 * @param {Object} action - Action data
 * @returns {Promise<Object>} Created action
 */
export const addAction = async (workflowId, action) => {
  try {
    return await api.post(`/api/v1/workflows/${workflowId}/actions`, action);
  } catch (error) {
    console.error(`Failed to add action to workflow ${workflowId}:`, error);
    throw error;
  }
};

/**
 * Update an action in a workflow
 * @param {string} workflowId - Workflow ID
 * @param {string} actionId - Action ID
 * @param {Object} action - Action data to update
 * @returns {Promise<Object>} Update result
 */
export const updateAction = async (workflowId, actionId, action) => {
  try {
    return await api.put(`/api/v1/workflows/${workflowId}/actions/${actionId}`, action);
  } catch (error) {
    console.error(`Failed to update action ${actionId} in workflow ${workflowId}:`, error);
    throw error;
  }
};

/**
 * Delete an action from a workflow
 * @param {string} workflowId - Workflow ID
 * @param {string} actionId - Action ID
 * @returns {Promise<Object>} Deletion result
 */
export const deleteAction = async (workflowId, actionId) => {
  try {
    return await api.delete(`/api/v1/workflows/${workflowId}/actions/${actionId}`);
  } catch (error) {
    console.error(`Failed to delete action ${actionId} from workflow ${workflowId}:`, error);
    throw error;
  }
};

/**
 * Reorder actions in a workflow
 * @param {string} workflowId - Workflow ID
 * @param {Array<string>} actionIds - Ordered list of action IDs
 * @returns {Promise<Object>} Reorder result
 */
export const reorderActions = async (workflowId, actionIds) => {
  try {
    return await api.put(`/api/v1/workflows/${workflowId}/actions/reorder`, { action_ids: actionIds });
  } catch (error) {
    console.error(`Failed to reorder actions in workflow ${workflowId}:`, error);
    throw error;
  }
};

/**
 * Add a data mapping to a workflow
 * @param {string} workflowId - Workflow ID
 * @param {Object} mapping - Data mapping
 * @returns {Promise<Object>} Created data mapping
 */
export const addDataMapping = async (workflowId, mapping) => {
  try {
    return await api.post(`/api/v1/workflows/${workflowId}/mappings`, mapping);
  } catch (error) {
    console.error(`Failed to add data mapping to workflow ${workflowId}:`, error);
    throw error;
  }
};

/**
 * Update a data mapping in a workflow
 * @param {string} workflowId - Workflow ID
 * @param {string} mappingId - Data mapping ID
 * @param {Object} mapping - Data mapping to update
 * @returns {Promise<Object>} Update result
 */
export const updateDataMapping = async (workflowId, mappingId, mapping) => {
  try {
    return await api.put(`/api/v1/workflows/${workflowId}/mappings/${mappingId}`, mapping);
  } catch (error) {
    console.error(`Failed to update data mapping ${mappingId} in workflow ${workflowId}:`, error);
    throw error;
  }
};

/**
 * Delete a data mapping from a workflow
 * @param {string} workflowId - Workflow ID
 * @param {string} mappingId - Data mapping ID
 * @returns {Promise<Object>} Deletion result
 */
export const deleteDataMapping = async (workflowId, mappingId) => {
  try {
    return await api.delete(`/api/v1/workflows/${workflowId}/mappings/${mappingId}`);
  } catch (error) {
    console.error(`Failed to delete data mapping ${mappingId} from workflow ${workflowId}:`, error);
    throw error;
  }
};

/**
 * Get workflow execution history
 * @param {string} workflowId - Workflow ID
 * @returns {Promise<Object>} Execution history
 */
export const getWorkflowExecutions = async (workflowId) => {
  try {
    return await api.get(`/api/v1/executions?workflow_id=${workflowId}`);
  } catch (error) {
    console.error(`Failed to get execution history for workflow ${workflowId}:`, error);
    throw error;
  }
};