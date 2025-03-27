import React, { useState, useEffect } from 'react';
import { useNotification } from '../../contexts/NotificationContext';
import Button from '../common/Button';
import { getFieldSchema } from '../../services/integration';

const DataMapperPanel = ({ 
  isOpen, 
  onClose, 
  workflow, 
  selectedMapping = null,
  onAddMapping,
  onUpdateMapping
}) => {
  const { showNotification } = useNotification();
  
  // Form state
  const [mapping, setMapping] = useState({
    source_service: workflow.trigger_service,
    source_field: '',
    target_service: '',
    target_field: '',
    transformer: ''
  });
  
  // Available fields
  const [sourceFields, setSourceFields] = useState([]);
  const [targetFields, setTargetFields] = useState([]);
  
  // Loading states
  const [loadingSourceFields, setLoadingSourceFields] = useState(false);
  const [loadingTargetFields, setLoadingTargetFields] = useState(false);
  const [submitting, setSubmitting] = useState(false);
  
  // Available transformers
  const transformers = [
    { id: '', name: 'None' },
    { id: 'toString', name: 'Convert to String' },
    { id: 'toNumber', name: 'Convert to Number' },
    { id: 'toBoolean', name: 'Convert to Boolean' },
    { id: 'toArray', name: 'Convert to Array' },
    { id: 'toLowerCase', name: 'Convert to Lowercase' },
    { id: 'toUpperCase', name: 'Convert to Uppercase' }
  ];
  
  // Target services - all unique action_service values from the workflow actions
  const targetServices = [...new Set(workflow.actions.map(action => action.action_service))];
  
  // Initialize form for editing if a mapping is selected
  useEffect(() => {
    if (selectedMapping) {
      setMapping({
        source_service: selectedMapping.source_service || workflow.trigger_service,
        source_field: selectedMapping.source_field || '',
        target_service: selectedMapping.target_service || '',
        target_field: selectedMapping.target_field || '',
        transformer: selectedMapping.transformer || ''
      });
    } else {
      setMapping({
        source_service: workflow.trigger_service,
        source_field: '',
        target_service: targetServices.length > 0 ? targetServices[0] : '',
        target_field: '',
        transformer: ''
      });
    }
  }, [selectedMapping, workflow.trigger_service, targetServices]);
  
  // Load source fields when source service changes
  useEffect(() => {
    const loadSourceFields = async () => {
      if (!mapping.source_service) return;
      
      setLoadingSourceFields(true);
      try {
        // Get fields for trigger
        const schema = await getFieldSchema(
          mapping.source_service, 
          'trigger', 
          workflow.trigger_id
        );
        
        // Extract fields from the output schema
        const fields = extractFieldsFromSchema(schema.output_schema);
        setSourceFields(fields);
      } catch (error) {
        console.error('Failed to load source fields:', error);
        showNotification('error', 'Failed to load source fields');
        setSourceFields([]);
      } finally {
        setLoadingSourceFields(false);
      }
    };
    
    loadSourceFields();
  }, [mapping.source_service, workflow.trigger_id, showNotification]);
  
  // Load target fields when target service changes
  useEffect(() => {
    const loadTargetFields = async () => {
      if (!mapping.target_service) return;
      
      setLoadingTargetFields(true);
      try {
        // Get action details for the selected service
        const action = workflow.actions.find(a => a.action_service === mapping.target_service);
        if (!action) {
          setTargetFields([]);
          return;
        }
        
        // Get fields for action
        const schema = await getFieldSchema(
          mapping.target_service, 
          'action', 
          action.action_id
        );
        
        // Extract fields from the input schema
        const fields = extractFieldsFromSchema(schema.input_schema);
        setTargetFields(fields);
      } catch (error) {
        console.error('Failed to load target fields:', error);
        showNotification('error', 'Failed to load target fields');
        setTargetFields([]);
      } finally {
        setLoadingTargetFields(false);
      }
    };
    
    loadTargetFields();
  }, [mapping.target_service, workflow.actions, showNotification]);
  
  // Handle form input changes
  const handleInputChange = (e) => {
    const { name, value } = e.target;
    setMapping(prev => ({
      ...prev,
      [name]: value
    }));
  };
  
  // Handle form submission
  const handleSubmit = async (e) => {
    e.preventDefault();
    
    // Validate form
    if (!mapping.source_field) {
      showNotification('error', 'Please select a source field');
      return;
    }
    
    if (!mapping.target_service) {
      showNotification('error', 'Please select a target service');
      return;
    }
    
    if (!mapping.target_field) {
      showNotification('error', 'Please select a target field');
      return;
    }
    
    setSubmitting(true);
    
    try {
      if (selectedMapping) {
        // Update existing mapping
        await onUpdateMapping(selectedMapping.id, mapping);
        showNotification('success', 'Data mapping updated successfully');
      } else {
        // Add new mapping
        await onAddMapping(mapping);
        showNotification('success', 'Data mapping added successfully');
      }
      
      // Close the panel
      onClose();
    } catch (error) {
      console.error('Failed to save data mapping:', error);
      showNotification('error', 'Failed to save data mapping');
    } finally {
      setSubmitting(false);
    }
  };
  
  // Extract fields from a JSON schema
  const extractFieldsFromSchema = (schema) => {
    try {
      const parsedSchema = typeof schema === 'string' ? JSON.parse(schema) : schema;
      
      if (!parsedSchema || !parsedSchema.properties) {
        return [];
      }
      
      // Extract top-level properties
      return Object.keys(parsedSchema.properties).map(key => ({
        id: key,
        name: key,
        type: parsedSchema.properties[key].type || 'string'
      }));
    } catch (error) {
      console.error('Failed to parse schema:', error);
      return [];
    }
  };
  
  if (!isOpen) return null;
  
  return (
    <div className="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50">
      <div className="bg-white rounded-lg shadow-xl p-6 w-full max-w-lg">
        <div className="flex justify-between items-center mb-4">
          <h2 className="text-xl font-semibold">
            {selectedMapping ? 'Edit Data Mapping' : 'Add Data Mapping'}
          </h2>
          <button
            onClick={onClose}
            className="text-gray-500 hover:text-gray-700"
          >
            <svg xmlns="http://www.w3.org/2000/svg" className="h-6 w-6" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" />
            </svg>
          </button>
        </div>
        
        <form onSubmit={handleSubmit}>
          {/* Source section */}
          <div className="mb-4">
            <h3 className="text-md font-medium mb-2">Source</h3>
            <div className="grid grid-cols-1 gap-3">
              <div>
                <label htmlFor="source_service" className="block text-sm font-medium text-gray-700 mb-1">
                  Service
                </label>
                <select
                  id="source_service"
                  name="source_service"
                  value={mapping.source_service}
                  onChange={handleInputChange}
                  disabled={true} // Only trigger is supported as source for now
                  className="w-full px-3 py-2 border border-gray-300 rounded-md bg-gray-100"
                >
                  <option value={workflow.trigger_service}>
                    {workflow.trigger_service} (Trigger)
                  </option>
                </select>
              </div>
              
              <div>
                <label htmlFor="source_field" className="block text-sm font-medium text-gray-700 mb-1">
                  Field
                </label>
                <select
                  id="source_field"
                  name="source_field"
                  value={mapping.source_field}
                  onChange={handleInputChange}
                  disabled={loadingSourceFields}
                  className="w-full px-3 py-2 border border-gray-300 rounded-md"
                >
                  <option value="">Select a field</option>
                  {sourceFields.map(field => (
                    <option key={field.id} value={field.id}>
                      {field.name} ({field.type})
                    </option>
                  ))}
                </select>
                {loadingSourceFields && (
                  <div className="mt-1 text-sm text-gray-500">Loading fields...</div>
                )}
              </div>
            </div>
          </div>
          
          {/* Target section */}
          <div className="mb-4">
            <h3 className="text-md font-medium mb-2">Target</h3>
            <div className="grid grid-cols-1 gap-3">
              <div>
                <label htmlFor="target_service" className="block text-sm font-medium text-gray-700 mb-1">
                  Service
                </label>
                <select
                  id="target_service"
                  name="target_service"
                  value={mapping.target_service}
                  onChange={handleInputChange}
                  className="w-full px-3 py-2 border border-gray-300 rounded-md"
                >
                  <option value="">Select a service</option>
                  {targetServices.map(service => (
                    <option key={service} value={service}>
                      {service}
                    </option>
                  ))}
                </select>
              </div>
              
              <div>
                <label htmlFor="target_field" className="block text-sm font-medium text-gray-700 mb-1">
                  Field
                </label>
                <select
                  id="target_field"
                  name="target_field"
                  value={mapping.target_field}
                  onChange={handleInputChange}
                  disabled={loadingTargetFields || !mapping.target_service}
                  className="w-full px-3 py-2 border border-gray-300 rounded-md"
                >
                  <option value="">Select a field</option>
                  {targetFields.map(field => (
                    <option key={field.id} value={field.id}>
                      {field.name} ({field.type})
                    </option>
                  ))}
                </select>
                {loadingTargetFields && (
                  <div className="mt-1 text-sm text-gray-500">Loading fields...</div>
                )}
              </div>
            </div>
          </div>
          
          {/* Transformer */}
          <div className="mb-6">
            <label htmlFor="transformer" className="block text-sm font-medium text-gray-700 mb-1">
              Data Transformer (Optional)
            </label>
            <select
              id="transformer"
              name="transformer"
              value={mapping.transformer}
              onChange={handleInputChange}
              className="w-full px-3 py-2 border border-gray-300 rounded-md"
            >
              {transformers.map(transformer => (
                <option key={transformer.id} value={transformer.id}>
                  {transformer.name}
                </option>
              ))}
            </select>
            <div className="mt-1 text-xs text-gray-500">
              Transformers help convert data from one format to another.
            </div>
          </div>
          
          {/* Buttons */}
          <div className="flex justify-end space-x-3">
            <Button
              type="button"
              variant="secondary"
              onClick={onClose}
              disabled={submitting}
            >
              Cancel
            </Button>
            <Button
              type="submit"
              variant="primary"
              loading={submitting}
            >
              {selectedMapping ? 'Update' : 'Add'} Mapping
            </Button>
          </div>
        </form>
      </div>
    </div>
  );
};

export default DataMapperPanel;