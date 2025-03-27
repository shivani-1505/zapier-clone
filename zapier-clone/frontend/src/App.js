import React, { useState, useEffect } from 'react';
import { BrowserRouter as Router, Routes, Route, Navigate } from 'react-router-dom';
import { AuthProvider, useAuth } from './contexts/AuthContext';
import { NotificationProvider } from './contexts/NotificationContext';

// Layouts
import DashboardLayout from './components/layout/DashboardLayout';
import AuthLayout from './components/layout/AuthLayout';

// Pages
import Dashboard from './pages/Dashboard';
import Connections from './pages/Connections';
import ConnectionDetails from './pages/ConnectionDetails';
import AddConnection from './pages/AddConnection';
import Workflows from './pages/Workflows';
import WorkflowBuilder from './pages/WorkflowBuilder';
import WorkflowDetails from './pages/WorkflowDetails';
import Executions from './pages/Executions';
import ExecutionDetails from './pages/ExecutionDetails';
import Settings from './pages/Settings';
import Login from './pages/Login';
import Register from './pages/Register';
import NotFound from './pages/NotFound';

// Protected route component
const ProtectedRoute = ({ children }) => {
  const { isAuthenticated, loading } = useAuth();
  
  if (loading) {
    return <div className="flex items-center justify-center h-screen">
      <div className="animate-spin rounded-full h-12 w-12 border-b-2 border-blue-500"></div>
    </div>;
  }
  
  if (!isAuthenticated) {
    return <Navigate to="/login" />;
  }
  
  return children;
};

// Public route component (redirects to dashboard if authenticated)
const PublicRoute = ({ children }) => {
  const { isAuthenticated, loading } = useAuth();
  
  if (loading) {
    return <div className="flex items-center justify-center h-screen">
      <div className="animate-spin rounded-full h-12 w-12 border-b-2 border-blue-500"></div>
    </div>;
  }
  
  if (isAuthenticated) {
    return <Navigate to="/dashboard" />;
  }
  
  return children;
};

function App() {
  return (
    <Router>
      <AuthProvider>
        <NotificationProvider>
          <Routes>
            {/* Public routes */}
            <Route path="/" element={<PublicRoute><AuthLayout><Login /></AuthLayout></PublicRoute>} />
            <Route path="/login" element={<PublicRoute><AuthLayout><Login /></AuthLayout></PublicRoute>} />
            <Route path="/register" element={<PublicRoute><AuthLayout><Register /></AuthLayout></PublicRoute>} />
            
            {/* Protected routes */}
            <Route path="/dashboard" element={
              <ProtectedRoute>
                <DashboardLayout>
                  <Dashboard />
                </DashboardLayout>
              </ProtectedRoute>
            } />
            
            <Route path="/connections" element={
              <ProtectedRoute>
                <DashboardLayout>
                  <Connections />
                </DashboardLayout>
              </ProtectedRoute>
            } />
            
            <Route path="/connections/add" element={
              <ProtectedRoute>
                <DashboardLayout>
                  <AddConnection />
                </DashboardLayout>
              </ProtectedRoute>
            } />
            
            <Route path="/connections/:id" element={
              <ProtectedRoute>
                <DashboardLayout>
                  <ConnectionDetails />
                </DashboardLayout>
              </ProtectedRoute>
            } />
            
            <Route path="/workflows" element={
              <ProtectedRoute>
                <DashboardLayout>
                  <Workflows />
                </DashboardLayout>
              </ProtectedRoute>
            } />
            
            <Route path="/workflows/new" element={
              <ProtectedRoute>
                <DashboardLayout>
                  <WorkflowBuilder />
                </DashboardLayout>
              </ProtectedRoute>
            } />
            
            <Route path="/workflows/:id" element={
              <ProtectedRoute>
                <DashboardLayout>
                  <WorkflowDetails />
                </DashboardLayout>
              </ProtectedRoute>
            } />
            
            <Route path="/workflows/:id/edit" element={
              <ProtectedRoute>
                <DashboardLayout>
                  <WorkflowBuilder />
                </DashboardLayout>
              </ProtectedRoute>
            } />
            
            <Route path="/executions" element={
              <ProtectedRoute>
                <DashboardLayout>
                  <Executions />
                </DashboardLayout>
              </ProtectedRoute>
            } />
            
            <Route path="/executions/:id" element={
              <ProtectedRoute>
                <DashboardLayout>
                  <ExecutionDetails />
                </DashboardLayout>
              </ProtectedRoute>
            } />
            
            <Route path="/settings" element={
              <ProtectedRoute>
                <DashboardLayout>
                  <Settings />
                </DashboardLayout>
              </ProtectedRoute>
            } />
            
            {/* OAuth callback routes */}
            <Route path="/oauth/callback/:service" element={
              <ProtectedRoute>
                <DashboardLayout>
                  <div className="flex flex-col items-center justify-center h-full">
                    <h2 className="text-2xl font-semibold mb-4">Processing OAuth Callback</h2>
                    <div className="animate-spin rounded-full h-12 w-12 border-b-2 border-blue-500"></div>
                  </div>
                </DashboardLayout>
              </ProtectedRoute>
            } />
            
            {/* 404 Not Found */}
            <Route path="*" element={<NotFound />} />
          </Routes>
        </NotificationProvider>
      </AuthProvider>
    </Router>
  );
}

export default App;