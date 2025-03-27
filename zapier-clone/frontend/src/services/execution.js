import api from './api';

/**
 * List workflow executions with optional filters
 * @param {Object} params - Query parameters
 * @param {string} params.workflow_id - Filter by workflow ID
 * @param {string} params.status - Filter by execution status
 * @param {number} params.limit - Limit the number of results
 * @param {number} params.offset - Offset for pagination
 * @returns {Promise<Object>} List of executions
 */
export const listExecutions = async (params = {}) => {
  try {
    return await api.get('/api/v1/executions', { params });
  } catch (error) {
    console.error('Failed to list executions:', error);
    throw error;
  }
};

/**
 * Get a specific execution by ID
 * @param {string} id - Execution ID
 * @returns {Promise<Object>} Execution details
 */
export const getExecution = async (id) => {
  try {
    return await api.get(`/api/v1/executions/${id}`);
  } catch (error) {
    console.error(`Failed to get execution ${id}:`, error);
    throw error;
  }
};

/**
 * Trigger a workflow execution manually
 * @param {string} workflowId - Workflow ID
 * @param {Object} triggerData - Trigger data
 * @returns {Promise<Object>} Created execution
 */
export const triggerExecution = async (workflowId, triggerData) => {
  try {
    return await api.post(`/api/v1/executions/workflow/${workflowId}`, { trigger_data: triggerData });
  } catch (error) {
    console.error(`Failed to trigger workflow ${workflowId}:`, error);
    throw error;
  }
};

/**
 * Get action executions for a workflow execution
 * @param {string} executionId - Execution ID
 * @returns {Promise<Object>} Action executions
 */
export const getActionExecutions = async (executionId) => {
  try {
    return await api.get(`/api/v1/executions/${executionId}/actions`);
  } catch (error) {
    console.error(`Failed to get action executions for ${executionId}:`, error);
    throw error;
  }
};

/**
 * Get execution logs
 * @param {string} executionId - Execution ID
 * @returns {Promise<Object>} Execution logs
 */
export const getExecutionLogs = async (executionId) => {
  try {
    return await api.get(`/api/v1/executions/${executionId}/logs`);
  } catch (error) {
    console.error(`Failed to get logs for execution ${executionId}:`, error);
    throw error;
  }
};

/**
 * Retry a failed execution
 * @param {string} executionId - Execution ID
 * @returns {Promise<Object>} Retry result
 */
export const retryExecution = async (executionId) => {
  try {
    return await api.post(`/api/v1/executions/${executionId}/retry`);
  } catch (error) {
    console.error(`Failed to retry execution ${executionId}:`, error);
    throw error;
  }
};

/**
 * Cancel a running execution
 * @param {string} executionId - Execution ID
 * @returns {Promise<Object>} Cancellation result
 */
export const cancelExecution = async (executionId) => {
  try {
    return await api.post(`/api/v1/executions/${executionId}/cancel`);
  } catch (error) {
    console.error(`Failed to cancel execution ${executionId}:`, error);
    throw error;
  }
};

/**
 * Get recent executions for a workflow
 * @param {string} workflowId - Workflow ID
 * @param {number} limit - Limit the number of results (default: 5)
 * @returns {Promise<Object>} Recent executions
 */
export const getRecentExecutions = async (workflowId, limit = 5) => {
  try {
    return await api.get(`/api/v1/executions`, {
      params: {
        workflow_id: workflowId,
        limit,
        sort: 'started_at:desc'
      }
    });
  } catch (error) {
    console.error(`Failed to get recent executions for workflow ${workflowId}:`, error);
    throw error;
  }
};

/**
 * Get execution statistics for a workflow
 * @param {string} workflowId - Workflow ID
 * @param {string} timeframe - Timeframe (day, week, month)
 * @returns {Promise<Object>} Execution statistics
 */
export const getExecutionStats = async (workflowId, timeframe = 'week') => {
  try {
    return await api.get(`/api/v1/executions/stats`, {
      params: {
        workflow_id: workflowId,
        timeframe
      }
    });
  } catch (error) {
    console.error(`Failed to get execution stats for workflow ${workflowId}:`, error);
    throw error;
  }
};

/**
 * Get execution detail with action executions
 * @param {string} executionId - Execution ID
 * @returns {Promise<Object>} Execution with action details
 */
export const getExecutionWithActions = async (executionId) => {
  try {
    const [execution, actionExecutions] = await Promise.all([
      getExecution(executionId),
      getActionExecutions(executionId)
    ]);

    return {
      ...execution,
      actions: actionExecutions
    };
  } catch (error) {
    console.error(`Failed to get execution with actions for ${executionId}:`, error);
    throw error;
  }
};