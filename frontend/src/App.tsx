import { QueryClient, QueryClientProvider } from "@tanstack/react-query"
import { BrowserRouter, Navigate, Route, Routes } from "react-router-dom"
import { Toaster } from "@/components/ui/sonner"
import { TooltipProvider } from "@/components/ui/tooltip"
import { AppLayout } from "@/components/shared/app-layout"
import { ProtectedRoute } from "@/components/shared/protected-route"
import { AuthProvider } from "@/lib/auth"
import { LoginPage } from "@/pages/login"
import { ProjectListPage } from "@/pages/project-list"
import { FlagsListPage } from "@/pages/flags-list"
import { FlagEditorPage } from "@/pages/flag-editor"
import { ParametersBrowserPage } from "@/pages/parameters-browser"
import { MembersListPage } from "@/pages/members-list"
import { ProjectSettingsPage } from "@/pages/project-settings"

const queryClient = new QueryClient()

function App() {
  return (
    <QueryClientProvider client={queryClient}>
      <TooltipProvider>
        <AuthProvider>
          <BrowserRouter>
            <Routes>
              <Route path="/login" element={<LoginPage />} />
              <Route element={<ProtectedRoute />}>
                <Route element={<AppLayout />}>
                  <Route path="/projects" element={<ProjectListPage />} />
                  <Route path="/projects/:projectId/flags" element={<FlagsListPage />} />
                <Route path="/projects/:projectId/flags/:key" element={<FlagEditorPage />} />
                  <Route path="/projects/:projectId/parameters" element={<ParametersBrowserPage />} />
                  <Route path="/projects/:projectId/members" element={<MembersListPage />} />
                <Route path="/projects/:projectId/settings" element={<ProjectSettingsPage />} />
                </Route>
              </Route>
              <Route path="/" element={<Navigate to="/projects" replace />} />
            </Routes>
          </BrowserRouter>
          <Toaster />
        </AuthProvider>
      </TooltipProvider>
    </QueryClientProvider>
  )
}

export default App
