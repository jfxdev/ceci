import { NavLink, useParams } from "react-router-dom"
import { Flag, KeyRound, Users, Settings, FolderKanban, type LucideIcon } from "lucide-react"

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

/** Flat project-scoped nav (Flags/Parameters/Members) — no nested sub-items needed for ceci. */
export function NavMain() {
  const { projectId } = useParams<{ projectId: string }>()
  if (!projectId) return null

  const items: NavItem[] = [
    { title: "Flags", path: `/projects/${projectId}/flags`, icon: Flag },
    { title: "Parameters", path: `/projects/${projectId}/parameters`, icon: KeyRound },
    { title: "Members", path: `/projects/${projectId}/members`, icon: Users },
    { title: "Settings", path: `/projects/${projectId}/settings`, icon: Settings },
  ]

  return (
    <SidebarGroup>
      <SidebarGroupLabel>Project</SidebarGroupLabel>
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
  )
}
