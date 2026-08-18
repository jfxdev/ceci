import { Link, Outlet, useLocation, useParams } from "react-router-dom"
import { TriangleAlert } from "lucide-react"

import { AppSidebar } from "@/components/app-sidebar"
import {
  Breadcrumb,
  BreadcrumbItem,
  BreadcrumbLink,
  BreadcrumbList,
  BreadcrumbPage,
  BreadcrumbSeparator,
} from "@/components/ui/breadcrumb"
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select"
import { Separator } from "@/components/ui/separator"
import { SidebarInset, SidebarProvider, SidebarTrigger } from "@/components/ui/sidebar"
import { EnvironmentProvider, useEnvironment } from "@/lib/environment"
import { useMaintenance } from "@/lib/maintenance"

const SECTION_LABEL: Record<string, string> = {
  overview: "Overview",
  flags: "Flags",
  parameters: "Parameters",
  members: "Members",
  environments: "Environments",
  settings: "Settings",
  "access-groups": "Access groups",
}

/** shadcn sidebar-07 dashboard shell wrapping every authenticated page. */
export function AppLayout() {
  const { projectId } = useParams<{ projectId: string }>()
  const location = useLocation()
  const section = location.pathname.split("/").pop() ?? ""
  const { status: maintenance } = useMaintenance()

  return (
    <SidebarProvider>
      <AppSidebar />
      <SidebarInset>
        <EnvironmentProvider key={projectId}>
          <header className="flex h-16 shrink-0 items-center gap-2 border-b px-4">
            <SidebarTrigger className="-ml-1" />
            <Separator orientation="vertical" className="mr-2 h-4" />
            <Breadcrumb>
              <BreadcrumbList>
                <BreadcrumbItem>
                  <BreadcrumbLink asChild>
                    <Link to="/projects">Projects</Link>
                  </BreadcrumbLink>
                </BreadcrumbItem>
                {location.pathname.startsWith("/admin/") ? (
                  <>
                    <BreadcrumbSeparator />
                    <BreadcrumbItem><BreadcrumbPage>{section === "access-groups" ? "Access groups" : "Admin settings"}</BreadcrumbPage></BreadcrumbItem>
                  </>
                ) : projectId && SECTION_LABEL[section] && (
                  <>
                    <BreadcrumbSeparator />
                    <BreadcrumbItem>
                      <BreadcrumbPage>{SECTION_LABEL[section]}</BreadcrumbPage>
                    </BreadcrumbItem>
                  </>
                )}
              </BreadcrumbList>
            </Breadcrumb>
            {projectId && (
              <div className="ml-auto">
                <EnvironmentSwitcher />
              </div>
            )}
          </header>
          {maintenance.enabled && (
            <div role="alert" className="flex items-start gap-3 border-b border-amber-300 bg-amber-50 px-4 py-3 text-amber-950 dark:border-amber-900 dark:bg-amber-950/40 dark:text-amber-100">
              <TriangleAlert className="mt-0.5 size-5 shrink-0" />
              <div className="text-sm leading-5">
                <p className="font-semibold">Maintenance mode is active</p>
                <p>{maintenance.message || "Maintenance is in progress. Changes are temporarily disabled."}</p>
              </div>
            </div>
          )}
          <Outlet />
        </EnvironmentProvider>
      </SidebarInset>
    </SidebarProvider>
  )
}

function EnvironmentSwitcher() {
  const { environments, envKey, setEnvKey, isLoading } = useEnvironment()

  if (isLoading || environments.length === 0) return null

  return (
    <Select key={envKey} value={envKey} onValueChange={setEnvKey}>
      <SelectTrigger className="w-40">
        <SelectValue placeholder="Environment" />
      </SelectTrigger>
      <SelectContent>
        {environments.map((e) => (
          <SelectItem key={e.key} value={e.key}>
            {e.name}
          </SelectItem>
        ))}
      </SelectContent>
    </Select>
  )
}
