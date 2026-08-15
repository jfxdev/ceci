import { QueryClient, QueryClientProvider } from "@tanstack/react-query"
import { BrowserRouter, Navigate, Route, Routes } from "react-router-dom"
import { Toaster } from "@/components/ui/sonner"
import { TooltipProvider } from "@/components/ui/tooltip"
import { AppLayout } from "@/components/shared/app-layout"
import { ProtectedRoute } from "@/components/shared/protected-route"
import { AuthProvider } from "@/lib/auth"
import { MaintenanceProvider } from "@/lib/maintenance"
import { LoginPage } from "@/pages/login"
import { RegisterPage } from "@/pages/register"
import { ProjectListPage } from "@/pages/project-list"
import { ProjectOverviewPage } from "@/pages/project-overview"
import { FlagsListPage } from "@/pages/flags-list"
import { FlagEditorPage } from "@/pages/flag-editor"
import { ParametersBrowserPage } from "@/pages/parameters-browser"
import { MembersListPage } from "@/pages/members-list"
import { ProjectSettingsPage } from "@/pages/project-settings"
import { PlaygroundPage } from "@/pages/playground"
import { EnvironmentListPage } from "@/pages/environment-list"
import { AdminSettingsPage } from "@/pages/admin-settings"
import { AccessGroupsPage } from "@/pages/access-groups"

const queryClient = new QueryClient()

function App() {
  return (
    <QueryClientProvider client={queryClient}>
      <TooltipProvider>
        <AuthProvider>
          <MaintenanceProvider>
            <BrowserRouter>
              <Routes>
              <Route path="/login" element={<LoginPage />} />
              <Route path="/register" element={<RegisterPage />} />
              <Route element={<ProtectedRoute />}>
                <Route element={<AppLayout />}>
                  <Route path="/projects" element={<ProjectListPage />} />
                  <Route path="/projects/:projectId/overview" element={<ProjectOverviewPage />} />
                  <Route path="/projects/:projectId/flags" element={<FlagsListPage />} />
                <Route path="/projects/:projectId/flags/:key" element={<FlagEditorPage />} />
                  <Route path="/projects/:projectId/parameters" element={<ParametersBrowserPage />} />
                  <Route path="/projects/:projectId/playground" element={<PlaygroundPage />} />
                  <Route path="/projects/:projectId/environments" element={<EnvironmentListPage />} />
                  <Route path="/projects/:projectId/members" element={<MembersListPage />} />
                <Route path="/projects/:projectId/settings" element={<ProjectSettingsPage />} />
                  <Route path="/admin/settings" element={<AdminSettingsPage />} />
                  <Route path="/admin/access-groups" element={<AccessGroupsPage />} />
                </Route>
              </Route>
              <Route path="/" element={<Navigate to="/projects" replace />} />
              </Routes>
            </BrowserRouter>
          </MaintenanceProvider>
          <Toaster />
        </AuthProvider>
      </TooltipProvider>
    </QueryClientProvider>
  )
}

export default App
