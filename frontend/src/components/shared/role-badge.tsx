import { Badge } from "@/components/ui/badge"
import { cn } from "@/lib/utils"

export type ProjectRole = "owner" | "admin" | "editor" | "viewer"

const ROLE_STYLES: Record<ProjectRole, string> = {
  owner: "bg-purple-100 text-purple-800 dark:bg-purple-900 dark:text-purple-200",
  admin: "bg-blue-100 text-blue-800 dark:bg-blue-900 dark:text-blue-200",
  editor: "bg-green-100 text-green-800 dark:bg-green-900 dark:text-green-200",
  viewer: "bg-gray-100 text-gray-800 dark:bg-gray-800 dark:text-gray-200",
}

export interface RoleBadgeProps {
  role: ProjectRole
  className?: string
}

export function RoleBadge({ role, className }: RoleBadgeProps) {
  return <Badge className={cn(ROLE_STYLES[role], className)}>{role}</Badge>
}
