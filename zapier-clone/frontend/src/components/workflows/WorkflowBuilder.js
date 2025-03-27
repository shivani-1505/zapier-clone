import React, { useState, useEffect, useCallback } from 'react';
import { useNavigate, useParams } from 'react-router-dom';
import { DndProvider } from 'react-dnd';
import { HTML5Backend } from 'react-dnd-html5-backend';
import { useNotification } from '../../contexts/NotificationContext';

// Services
import { getWorkflow, createWorkflow, updateWorkflow, addAction, updateAction, deleteAction, reorderActions, addDataMapping, updateDataMapping, deleteDataMapping } from '../../services/workflow';
import { listConnections } from '../../services/connection';
import { listIntegrations, getIntegrationDetails } from '../../services/integration';

// Components
import Button from '../common/Button';
import Spinner from '../common/Loader';
import TriggerSelector from './TriggerSelector';
import ActionSelector from './ActionSelector';
import ActionCard from './ActionCard';
import DataMapperPanel from './DataMapper';
import ConfirmationModal from '../common/ConfirmationModal';

const WorkflowBuilder = () => {
  const navigate = useNavigate();
  const { id } = useParams();
  const { showNotification } = useNotification();
  const isEditMode = Boolean(id);

  // Workflow state
  const [workflow, setWorkflow] = useState({
    name: '',
    description: '',
    trigger_service: '',
    trigger_id: '',
    trigger_config: {},
    actions: [],
    data_mappings: []
  });

  // UI state
  const [loading, setLoading] = useState(false);
  const [savingWorkflow, setSavingWorkflow] = useState(false);
  const [connections, setConnections] = useState([]);
  const [integrations, setIntegrations] = useState([]);
  const [integrationDetails, setIntegrationDetails] = useState({});
  const [selectedAction, setSelectedAction] = useState(null);
  const [isActionModalOpen, setIsActionModalOpen] = useState(false);
  const [isDataMapperOpen, setIsDataMapperOpen] = useState(false);
  const [isDragging, setIsDragging] = useState(false);
  const [confirmationModal, setConfirmationModal] = useState({ 
    isOpen: false, 
    title: '', 
    message: '', 
    confirmAction: null 
  });

  // Load workflow data if in edit mode
  useEffect(() => {
    const loadWorkflowData = async () => {
      if (isEditMode) {
        setLoading(true);
        try {
          const data = await getWorkflow(id);
          setWorkflow(data);
        } catch (error) {
          console.error('Failed to load workflow:', error);
          showNotification('error', 'Failed to load workflow data');
          navigate('/workflows');
        } finally {
          setLoading(false);
        }
      }
    };

    loadWorkflowData();
  }, [id, isEditMode, navigate, showNotification]);

  // Load connections and integrations
  useEffect(() => {
    const loadData = async () => {
      setLoading(true);
      try {
        const [connectionsData, integrationsData] = await Promise.all([
          listConnections(),
          listIntegrations()
        ]);
        
        setConnections(connectionsData.connections || []);
        setIntegrations(integrationsData.services || []);
      } catch (error) {
        console.error('Failed to load data:', error);
        showNotification('error', 'Failed to load required data');
      } finally {
        setLoading(false);
      }
    };

    loadData();
  }, [showNotification]);

  // Load integration details when trigger service changes
  useEffect(() => {
    const loadIntegrationDetails = async () => {
      if (!workflow.trigger_service) return;
      
      try {
        if (!integrationDetails[workflow.trigger_service]) {
          const details = await getIntegrationDetails(workflow.trigger_service);
          setIntegrationDetails(prev => ({
            ...prev,
            [workflow.trigger_service]: details
          }));
        }
      } catch (error) {
        console.error(`Failed to load ${workflow.trigger_service} details:`, error);
      }
    };

    loadIntegrationDetails();
  }, [workflow.trigger_service, integrationDetails]);

  // Handle form input changes
  const handleInputChange = (e) => {
    const { name, value } = e.target;
    setWorkflow(prev => ({
      ...prev,
      [name]: value
    }));
  };

  // Handle trigger selection
  const handleTriggerSelect = useCallback((service, triggerId, config) => {
    setWorkflow(prev => ({
      ...prev,
      trigger_service: service,
      trigger_id: triggerId,
      trigger_config: config
    }));
  }, []);

  // Handle adding an action
  const handleAddAction = useCallback(async (action) => {
    if (!isEditMode) {
      // In create mode, just add to local state
      setWorkflow(prev => ({
        ...prev,
        actions: [
          ...prev.actions,
          {
            id: `temp_${Date.now()}`,
            action_service: action.service,
            action_id: action.id,
            action_config: action.config,
            position: prev.actions.length + 1
          }
        ]
      }));
      return;
    }

    // In edit mode, call API
    try {
      const newAction = await addAction(id, {
        action_service: action.service,
        action_id: action.id,
        action_config: action.config,
        position: workflow.actions.length + 1
      });

      setWorkflow(prev => ({
        ...prev,
        actions: [...prev.actions, newAction]
      }));

      showNotification('success', 'Action added successfully');
    } catch (error) {
      console.error('Failed to add action:', error);
      showNotification('error', 'Failed to add action');
    }
  }, [id, isEditMode, workflow.actions, showNotification]);

  // Handle editing an action
  const handleEditAction = useCallback(async (actionId, updates) => {
    if (!isEditMode) {
      // In create mode, update local state
      setWorkflow(prev => ({
        ...prev,
        actions: prev.actions.map(action => 
          action.id === actionId 
            ? { ...action, ...updates } 
            : action
        )
      }));
      return;
    }

    // In edit mode, call API
    try {
      await updateAction(id, actionId, updates);
      
      setWorkflow(prev => ({
        ...prev,
        actions: prev.actions.map(action => 
          action.id === actionId 
            ? { ...action, ...updates } 
            : action
        )
      }));

      showNotification('success', 'Action updated successfully');
    } catch (error) {
      console.error('Failed to update action:', error);
      showNotification('error', 'Failed to update action');
    }
  }, [id, isEditMode, showNotification]);

  // Handle deleting an action
  const handleDeleteAction = useCallback(async (actionId) => {
    setConfirmationModal({
      isOpen: true,
      title: 'Delete Action',
      message: 'Are you sure you want to delete this action? This cannot be undone.',
      confirmAction: async () => {
        if (!isEditMode) {
          // In create mode, update local state
          setWorkflow(prev => ({
            ...prev,
            actions: prev.actions.filter(action => action.id !== actionId)
              .map((action, index) => ({ ...action, position: index + 1 }))
          }));
          return;
        }

        // In edit mode, call API
        try {
          await deleteAction(id, actionId);
          
          setWorkflow(prev => ({
            ...prev,
            actions: prev.actions.filter(action => action.id !== actionId)
              .map((action, index) => ({ ...action, position: index + 1 }))
          }));

          showNotification('success', 'Action deleted successfully');
        } catch (error) {
          console.error('Failed to delete action:', error);
          showNotification('error', 'Failed to delete action');
        }
      }
    });
  }, [id, isEditMode, showNotification]);

  // Handle reordering actions
  const handleReorderActions = useCallback(async (sourceIndex, destinationIndex) => {
    // Update local state first
    setWorkflow(prev => {
      const newActions = [...prev.actions];
      const [movedAction] = newActions.splice(sourceIndex, 1);
      newActions.splice(destinationIndex, 0, movedAction);
      
      // Update positions
      return {
        ...prev,
        actions: newActions.map((action, index) => ({
          ...action,
          position: index + 1
        }))
      };
    });

    if (isEditMode) {
      // In edit mode, call API
      try {
        await reorderActions(id, workflow.actions.map(action => action.id));
      } catch (error) {
        console.error('Failed to reorder actions:', error);
        showNotification('error', 'Failed to reorder actions');
      }
    }
  }, [id, isEditMode, workflow.actions, showNotification]);

  // Handle adding a data mapping
  const handleAddDataMapping = useCallback(async (mapping) => {
    if (!isEditMode) {
      // In create mode, just add to local state
      setWorkflow(prev => ({
        ...prev,
        data_mappings: [
          ...prev.data_mappings,
          {
            id: `temp_${Date.now()}`,
            ...mapping
          }
        ]
      }));
      return;
    }

    // In edit mode, call API
    try {
      const newMapping = await addDataMapping(id, mapping);

      setWorkflow(prev => ({
        ...prev,
        data_mappings: [...prev.data_mappings, newMapping]
      }));

      showNotification('success', 'Data mapping added successfully');
    } catch (error) {
      console.error('Failed to add data mapping:', error);
      showNotification('error', 'Failed to add data mapping');
    }
  }, [id, isEditMode, showNotification]);

  // Handle updating a data mapping
  const handleUpdateDataMapping = useCallback(async (mappingId, updates) => {
    if (!isEditMode) {
      // In create mode, update local state
      setWorkflow(prev => ({
        ...prev,
        data_mappings: prev.data_mappings.map(mapping => 
          mapping.id === mappingId 
            ? { ...mapping, ...updates } 
            : mapping
        )
      }));
      return;
    }

    // In edit mode, call API
    try {
      await updateDataMapping(id, mappingId, updates);
      
      setWorkflow(prev => ({
        ...prev,
        data_mappings: prev.data_mappings.map(mapping => 
          mapping.id === mappingId 
            ? { ...mapping, ...updates } 
            : mapping
        )
      }));

      showNotification('success', 'Data mapping updated successfully');
    } catch (error) {
      console.error('Failed to update data mapping:', error);
      showNotification('error', 'Failed to update data mapping');
    }
  }, [id, isEditMode, showNotification]);

  // Handle deleting a data mapping
  const handleDeleteDataMapping = useCallback(async (mappingId) => {
    if (!isEditMode) {
      // In create mode, update local state
      setWorkflow(prev => ({
        ...prev,
        data_mappings: prev.data_mappings.filter(mapping => mapping.id !== mappingId)
      }));
      return;
    }

    // In edit mode, call API
    try {
      await deleteDataMapping(id, mappingId);
      
      setWorkflow(prev => ({
        ...prev,
        data_mappings: prev.data_mappings.filter(mapping => mapping.id !== mappingId)
      }));

      showNotification('success', 'Data mapping deleted successfully');
    } catch (error) {
      console.error('Failed to delete data mapping:', error);
      showNotification('error', 'Failed to delete data mapping');
    }
  }, [id, isEditMode, showNotification]);

  // Save workflow
  const handleSaveWorkflow = async () => {
    // Validate form
    if (!workflow.name) {
      showNotification('error', 'Workflow name is required');
      return;
    }

    if (!workflow.trigger_service || !workflow.trigger_id) {
      showNotification('error', 'Please select a trigger');
      return;
    }

    if (workflow.actions.length === 0) {
      showNotification('error', 'Please add at least one action');
      return;
    }

    setSavingWorkflow(true);

    try {
      if (isEditMode) {
        // Update existing workflow
        await updateWorkflow(id, {
          name: workflow.name,
          description: workflow.description,
          trigger_service: workflow.trigger_service,
          trigger_id: workflow.trigger_id,
          trigger_config: workflow.trigger_config
        });
        
        showNotification('success', 'Workflow updated successfully');
      } else {
        // Create new workflow
        const newWorkflow = await createWorkflow({
          name: workflow.name,
          description: workflow.description,
          trigger_service: workflow.trigger_service,
          trigger_id: workflow.trigger_id,
          trigger_config: workflow.trigger_config
        });
        
        // Add all actions
        for (const action of workflow.actions) {
          await addAction(newWorkflow.id, {
            action_service: action.action_service,
            action_id: action.action_id,
            action_config: action.action_config,
            position: action.position
          });
        }
        
        // Add all data mappings
        for (const mapping of workflow.data_mappings) {
          await addDataMapping(newWorkflow.id, {
            source_service: mapping.source_service,
            source_field: mapping.source_field,
            target_service: mapping.target_service,
            target_field: mapping.target_field,
            transformer: mapping.transformer
          });
        }
        
        showNotification('success', 'Workflow created successfully');
        navigate(`/workflows/${newWorkflow.id}`);
      }
    } catch (error) {
      console.error('Failed to save workflow:', error);
      showNotification('error', 'Failed to save workflow');
    } finally {
      setSavingWorkflow(false);
    }
  };

  // Cancel workflow creation/editing
  const handleCancel = () => {
    navigate(isEditMode ? `/workflows/${id}` : '/workflows');
  };

  if (loading) {
    return (
      <div className="flex justify-center items-center h-64">
        <Spinner size="lg" />
      </div>
    );
  }

  return (
    <DndProvider backend={HTML5Backend}>
      <div className="container mx-auto px-4 py-6">
        <div className="flex justify-between items-center mb-6">
          <h1 className="text-2xl font-bold">
            {isEditMode ? 'Edit Workflow' : 'Create Workflow'}
          </h1>
          <div className="flex space-x-4">
            <Button 
              variant="secondary" 
              onClick={handleCancel}
              disabled={savingWorkflow}
            >
              Cancel
            </Button>
            <Button 
              variant="primary" 
              onClick={handleSaveWorkflow}
              loading={savingWorkflow}
            >
              {isEditMode ? 'Update Workflow' : 'Create Workflow'}
            </Button>
          </div>
        </div>

        {/* Basic Info */}
        <div className="bg-white rounded-lg shadow-md p-6 mb-6">
          <h2 className="text-xl font-semibold mb-4">Basic Information</h2>
          <div className="grid grid-cols-1 gap-6">
            <div>
              <label htmlFor="name" className="block text-sm font-medium text-gray-700 mb-1">
                Workflow Name *
              </label>
              <input
                type="text"
                id="name"
                name="name"
                value={workflow.name}
                onChange={handleInputChange}
                className="w-full px-3 py-2 border border-gray-300 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500"
                placeholder="Enter workflow name"
                required
              />
            </div>
            <div>
              <label htmlFor="description" className="block text-sm font-medium text-gray-700 mb-1">
                Description
              </label>
              <textarea
                id="description"
                name="description"
                value={workflow.description}
                onChange={handleInputChange}
                className="w-full px-3 py-2 border border-gray-300 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500"
                placeholder="Enter workflow description"
                rows="3"
              />
            </div>
          </div>
        </div>

        {/* Trigger */}
        <div className="bg-white rounded-lg shadow-md p-6 mb-6">
          <h2 className="text-xl font-semibold mb-4">Trigger</h2>
          <TriggerSelector
            integrations={integrations}
            connections={connections}
            selectedService={workflow.trigger_service}
            selectedTriggerId={workflow.trigger_id}
            selectedConfig={workflow.trigger_config}
            onSelectTrigger={handleTriggerSelect}
          />
        </div>

        {/* Actions */}
        <div className="bg-white rounded-lg shadow-md p-6 mb-6">
          <div className="flex justify-between items-center mb-4">
            <h2 className="text-xl font-semibold">Actions</h2>
            <Button 
              variant="primary" 
              onClick={() => setIsActionModalOpen(true)}
            >
              Add Action
            </Button>
          </div>

          {workflow.actions.length === 0 ? (
            <div className="bg-gray-100 rounded-lg p-8 text-center">
              <p className="text-gray-500">
                No actions added yet. Click the "Add Action" button to add your first action.
              </p>
            </div>
          ) : (
            <div className="space-y-4">
              {workflow.actions.map((action, index) => (
                <ActionCard
                  key={action.id}
                  action={action}
                  index={index}
                  isEditMode={isEditMode}
                  onEdit={(updates) => handleEditAction(action.id, updates)}
                  onDelete={() => handleDeleteAction(action.id)}
                  onMove={handleReorderActions}
                  isDragging={isDragging}
                  setIsDragging={setIsDragging}
                />
              ))}
            </div>
          )}
        </div>

        {/* Data Mapping */}
        {workflow.trigger_service && workflow.actions.length > 0 && (
          <div className="bg-white rounded-lg shadow-md p-6 mb-6">
            <div className="flex justify-between items-center mb-4">
              <h2 className="text-xl font-semibold">Data Mapping</h2>
              <Button 
                variant="primary" 
                onClick={() => setIsDataMapperOpen(true)}
              >
                Configure Data Mapping
              </Button>
            </div>

            {workflow.data_mappings.length === 0 ? (
              <div className="bg-gray-100 rounded-lg p-8 text-center">
                <p className="text-gray-500">
                  No data mappings configured yet. Click the "Configure Data Mapping" button to set up data flow between trigger and actions.
                </p>
              </div>
            ) : (
              <div className="overflow-x-auto">
                <table className="min-w-full divide-y divide-gray-200">
                  <thead className="bg-gray-50">
                    <tr>
                      <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Source</th>
                      <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Field</th>
                      <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Target</th>
                      <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Field</th>
                      <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Transformer</th>
                      <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Actions</th>
                    </tr>
                  </thead>
                  <tbody className="bg-white divide-y divide-gray-200">
                    {workflow.data_mappings.map((mapping) => (
                      <tr key={mapping.id}>
                        <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-900">{mapping.source_service}</td>
                        <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-900">{mapping.source_field}</td>
                        <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-900">{mapping.target_service}</td>
                        <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-900">{mapping.target_field}</td>
                        <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-900">{mapping.transformer || '-'}</td>
                        <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-500">
                          <Button 
                            variant="text" 
                            onClick={() => {
                              setSelectedAction(mapping);
                              setIsDataMapperOpen(true);
                            }}
                          >
                            Edit
                          </Button>
                          <Button 
                            variant="text" 
                            className="text-red-600"
                            onClick={() => handleDeleteDataMapping(mapping.id)}
                          >
                            Delete
                          </Button>
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            )}
          </div>
        )}

        {/* Action Modal */}
        {isActionModalOpen && (
          <ActionSelector
            isOpen={isActionModalOpen}
            onClose={() => setIsActionModalOpen(false)}
            integrations={integrations}
            connections={connections}
            onSelectAction={handleAddAction}
          />
        )}

        {/* Data Mapper Modal */}
        {isDataMapperOpen && (
          <DataMapperPanel
            isOpen={isDataMapperOpen}
            onClose={() => {
              setIsDataMapperOpen(false);
              setSelectedAction(null);
            }}
            workflow={workflow}
            selectedMapping={selectedAction}
            onAddMapping={handleAddDataMapping}
            onUpdateMapping={handleUpdateDataMapping}
          />
        )}

        {/* Confirmation Modal */}
        <ConfirmationModal
          isOpen={confirmationModal.isOpen}
          title={confirmationModal.title}
          message={confirmationModal.message}
          onConfirm={() => {
            confirmationModal.confirmAction();
            setConfirmationModal(prev => ({ ...prev, isOpen: false }));
          }}
          onCancel={() => setConfirmationModal(prev => ({ ...prev, isOpen: false }))}
        />
      </div>
    </DndProvider>
  );
};

export default WorkflowBuilder;