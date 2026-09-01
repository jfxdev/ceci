import { Link, Outlet, useLocation, useParams } from "react-router-dom"
import { Palette, TriangleAlert } from "lucide-react"

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
import { colorThemes, useColorTheme } from "@/lib/color-theme"
import { useMaintenance } from "@/lib/maintenance"
import { useTranslation } from "react-i18next"

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
  const { t } = useTranslation()
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
                    <Link to="/projects">{t("projects.title")}</Link>
                  </BreadcrumbLink>
                </BreadcrumbItem>
                {location.pathname.startsWith("/admin/") ? (
                  <>
                    <BreadcrumbSeparator />
                    <BreadcrumbItem><BreadcrumbPage>{section === "access-groups" ? t("nav.accessGroups") : t("nav.adminSettings")}</BreadcrumbPage></BreadcrumbItem>
                  </>
                ) : projectId && SECTION_LABEL[section] && (
                  <>
                    <BreadcrumbSeparator />
                    <BreadcrumbItem>
                      <BreadcrumbPage>{t(`nav.${section === "environments" ? "environments" : section}`)}</BreadcrumbPage>
                    </BreadcrumbItem>
                  </>
                )}
              </BreadcrumbList>
            </Breadcrumb>
            <div className="ml-auto flex shrink-0 items-center gap-2">
              <ColorThemeSwitcher />
              {projectId && <EnvironmentSwitcher />}
            </div>
          </header>
          {maintenance.enabled && (
            <div role="alert" className="flex items-start gap-3 border-b border-amber-300 bg-amber-50 px-4 py-3 text-amber-950 dark:border-amber-900 dark:bg-amber-950/40 dark:text-amber-100">
              <TriangleAlert className="mt-0.5 size-5 shrink-0" />
              <div className="text-sm leading-5">
                <p className="font-semibold">{t("pages.maintenanceMode")}</p>
                <p>{maintenance.message || t("pages.maintenanceDisabledDescription")}</p>
              </div>
            </div>
          )}
          <Outlet />
        </EnvironmentProvider>
      </SidebarInset>
    </SidebarProvider>
  )
}

function ColorThemeSwitcher() {
	const { t } = useTranslation()
  const { colorTheme, setColorTheme } = useColorTheme()
  const selectedTheme = colorThemes.find((theme) => theme.value === colorTheme) ?? colorThemes[0]

  return (
    <Select value={colorTheme} onValueChange={(value) => {
      const selectedTheme = colorThemes.find((theme) => theme.value === value)
      if (selectedTheme) setColorTheme(selectedTheme.value)
    }}>
	      <SelectTrigger className="w-9 px-2" showChevron={false} aria-label={`${t("pages.colorTheme")}: ${t(selectedTheme.labelKey)}`}>
        <Palette style={{ color: selectedTheme.swatches[0] }} aria-hidden="true" />
        <SelectValue className="sr-only" />
      </SelectTrigger>
      <SelectContent>
        {colorThemes.map((theme) => (
          <SelectItem key={theme.value} value={theme.value}>
            <span className="flex items-center gap-2">
              <span className="flex items-center gap-1" aria-hidden="true">
                {theme.swatches.map((color) => <span key={color} className="size-3 rounded-full border border-black/10" style={{ backgroundColor: color }} />)}
              </span>
	              {t(theme.labelKey)}
            </span>
          </SelectItem>
        ))}
      </SelectContent>
    </Select>
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
