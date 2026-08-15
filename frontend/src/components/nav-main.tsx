import { NavLink, useParams } from "react-router-dom"
import { LayoutDashboard, Flag, KeyRound, Users, Settings, FolderKanban, FlaskConical, Boxes, ShieldCheck, UsersRound, type LucideIcon } from "lucide-react"
import { useAuth } from "@/lib/auth"

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

  const items: NavItem[] = projectId ? [
    { title: "Overview", path: `/projects/${projectId}/overview`, icon: LayoutDashboard },
    { title: "Flags", path: `/projects/${projectId}/flags`, icon: Flag },
    { title: "Parameters", path: `/projects/${projectId}/parameters`, icon: KeyRound },
    { title: "Playground", path: `/projects/${projectId}/playground`, icon: FlaskConical },
    { title: "Environments", path: `/projects/${projectId}/environments`, icon: Boxes },
    { title: "Members", path: `/projects/${projectId}/members`, icon: Users },
    { title: "Settings", path: `/projects/${projectId}/settings`, icon: Settings },
  ] : []

  return (
    <>
      <SidebarGroup>
        {projectId && <SidebarGroupLabel>Project</SidebarGroupLabel>}
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
            <SidebarMenuButton asChild tooltip="All projects">
              <NavLink to="/projects">
                <FolderKanban />
                <span>All projects</span>
              </NavLink>
            </SidebarMenuButton>
          </SidebarMenuItem>
        </SidebarMenu>
      </SidebarGroup>
      {user?.isAdmin && (
        <SidebarGroup>
          <SidebarGroupLabel>Administration</SidebarGroupLabel>
          <SidebarMenu>
            <SidebarMenuItem>
              <SidebarMenuButton asChild tooltip="Admin settings">
                <NavLink to="/admin/settings" className={({ isActive }) => (isActive ? "font-medium" : undefined)}>
                  <ShieldCheck />
                  <span>Admin settings</span>
                </NavLink>
              </SidebarMenuButton>
            </SidebarMenuItem>
            <SidebarMenuItem>
              <SidebarMenuButton asChild tooltip="Access groups">
                <NavLink to="/admin/access-groups" className={({ isActive }) => (isActive ? "font-medium" : undefined)}>
                  <UsersRound />
                  <span>Access groups</span>
                </NavLink>
              </SidebarMenuButton>
            </SidebarMenuItem>
          </SidebarMenu>
        </SidebarGroup>
      )}
    </>
  )
}
