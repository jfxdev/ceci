import { NavLink, useParams } from "react-router-dom"
import { LayoutDashboard, Flag, KeyRound, Users, Settings, FolderKanban, FlaskConical, Boxes, ShieldCheck, UsersRound, Braces, type LucideIcon } from "lucide-react"
import { useAuth } from "@/lib/auth"
import { useTranslation } from "react-i18next"

import {
  SidebarGroup,
  SidebarGroupLabel,
  SidebarMenu,
  SidebarMenuButton,
  SidebarMenuItem,
} from "@/components/ui/sidebar"

interface NavItem {
  title: string
  path: string
  icon: LucideIcon
}

/** Flat project-scoped nav (Flags/Parameters/Members) — no nested sub-items needed for leaflag. */
export function NavMain() {
  const { projectId } = useParams<{ projectId: string }>()
  const { user } = useAuth()
  const { t } = useTranslation()

  const items: NavItem[] = projectId ? [
    { title: t("nav.overview"), path: `/projects/${projectId}/overview`, icon: LayoutDashboard },
    { title: t("nav.flags"), path: `/projects/${projectId}/flags`, icon: Flag },
    { title: t("nav.parameters"), path: `/projects/${projectId}/parameters`, icon: KeyRound },
    { title: t("nav.playground"), path: `/projects/${projectId}/playground`, icon: FlaskConical },
    { title: t("nav.environments"), path: `/projects/${projectId}/environments`, icon: Boxes },
    { title: t("nav.contexts"), path: `/projects/${projectId}/contexts`, icon: Braces },
    { title: t("nav.members"), path: `/projects/${projectId}/members`, icon: Users },
    { title: t("nav.settings"), path: `/projects/${projectId}/settings`, icon: Settings },
  ] : []

  return (
    <>
      <SidebarGroup>
        {projectId && <SidebarGroupLabel>{t("nav.project")}</SidebarGroupLabel>}
        <SidebarMenu>
          {items.map((item) => (
            <SidebarMenuItem key={item.path}>
              <SidebarMenuButton asChild tooltip={item.title}>
                <NavLink to={item.path} className={({ isActive }) => (isActive ? "font-medium" : undefined)}>
                  <item.icon />
                  <span>{item.title}</span>
                </NavLink>
              </SidebarMenuButton>
            </SidebarMenuItem>
          ))}
          <SidebarMenuItem>
            <SidebarMenuButton asChild tooltip={t("nav.allProjects")}>
              <NavLink to="/projects">
                <FolderKanban />
                <span>{t("nav.allProjects")}</span>
              </NavLink>
            </SidebarMenuButton>
          </SidebarMenuItem>
        </SidebarMenu>
      </SidebarGroup>
      {user?.isAdmin && (
        <SidebarGroup>
          <SidebarGroupLabel>{t("nav.administration")}</SidebarGroupLabel>
          <SidebarMenu>
            <SidebarMenuItem>
              <SidebarMenuButton asChild tooltip={t("nav.adminSettings")}>
                <NavLink to="/admin/settings" className={({ isActive }) => (isActive ? "font-medium" : undefined)}>
                  <ShieldCheck />
                  <span>{t("nav.adminSettings")}</span>
                </NavLink>
              </SidebarMenuButton>
            </SidebarMenuItem>
            <SidebarMenuItem>
              <SidebarMenuButton asChild tooltip={t("nav.accessGroups")}>
                <NavLink to="/admin/access-groups" className={({ isActive }) => (isActive ? "font-medium" : undefined)}>
                  <UsersRound />
                  <span>{t("nav.accessGroups")}</span>
                </NavLink>
              </SidebarMenuButton>
            </SidebarMenuItem>
          </SidebarMenu>
        </SidebarGroup>
      )}
    </>
  )
}
