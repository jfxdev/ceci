import { useState, type FormEvent } from "react"
import { useParams } from "react-router-dom"
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select"
import { Dialog, DialogContent, DialogFooter, DialogHeader, DialogTitle, DialogTrigger } from "@/components/ui/dialog"
import { RoleBadge, type ProjectRole } from "@/components/shared/role-badge"
import { DataTable, type DataTableColumn } from "@/components/shared/data-table"
import { ConfirmDialog } from "@/components/shared/confirm-dialog"
import { api } from "@/lib/api"
import { alertMessageFromError } from "@/lib/alerts"
import { useTranslation } from "react-i18next"

interface Member {
  userId: string
  email: string
  name: string
  role: ProjectRole
}

interface AccessGroup { id: string; name: string; description: string }
interface ProjectAccessGroup { groupId: string; name: string; description: string; role: ProjectRole }

const ROLES: ProjectRole[] = ["viewer", "editor", "admin", "owner"]

export function MembersListPage() {
  const { t } = useTranslation()
  const { projectId } = useParams<{ projectId: string }>()
  const queryClient = useQueryClient()
  const [open, setOpen] = useState(false)
  const [email, setEmail] = useState("")
  const [role, setRole] = useState<ProjectRole>("viewer")
  const [error, setError] = useState<string | null>(null)
  const [pendingRemove, setPendingRemove] = useState<Member | null>(null)
  const [groupOpen, setGroupOpen] = useState(false)
  const [groupId, setGroupId] = useState("")
  const [groupRole, setGroupRole] = useState<ProjectRole>("viewer")

  const { data: members = [], isLoading } = useQuery({
    queryKey: ["members", projectId],
    queryFn: () => api.get<Member[]>(`/projects/${projectId}/members`),
    enabled: !!projectId,
  })
  const { data: availableGroups = [] } = useQuery({
    queryKey: ["access-groups"],
    queryFn: () => api.get<AccessGroup[]>("/admin/access-groups"),
  })
  const { data: projectGroups = [], isLoading: isLoadingGroups } = useQuery({
    queryKey: ["project-access-groups", projectId],
    queryFn: () => api.get<ProjectAccessGroup[]>(`/projects/${projectId}/access-groups`),
    enabled: !!projectId,
  })

  const addMember = useMutation({
    mutationFn: () => api.post(`/projects/${projectId}/members`, { email, role }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["members", projectId] })
      setOpen(false)
      setEmail("")
      setRole("viewer")
    },
	    onError: (err) => setError(alertMessageFromError(err, t("pages.failedToAddMember"))),
  })

  const updateRole = useMutation({
    mutationFn: ({ userId, role }: { userId: string; role: ProjectRole }) =>
      api.patch(`/projects/${projectId}/members/${userId}`, { role }),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ["members", projectId] }),
  })

  const removeMember = useMutation({
    mutationFn: (userId: string) => api.delete(`/projects/${projectId}/members/${userId}`),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["members", projectId] })
      setPendingRemove(null)
    },
  })
  const linkGroup = useMutation({
    mutationFn: () => api.post(`/projects/${projectId}/access-groups`, { groupId, role: groupRole }),
    onSuccess: () => { queryClient.invalidateQueries({ queryKey: ["project-access-groups", projectId] }); setGroupId(""); setGroupRole("viewer"); setGroupOpen(false) },
	    onError: (err) => setError(alertMessageFromError(err, t("pages.failedToLinkGroup"))),
  })
  const updateGroupRole = useMutation({
    mutationFn: ({ id, role }: { id: string; role: ProjectRole }) => api.patch(`/projects/${projectId}/access-groups/${id}`, { role }),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ["project-access-groups", projectId] }),
  })
  const unlinkGroup = useMutation({
    mutationFn: (id: string) => api.delete(`/projects/${projectId}/access-groups/${id}`),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ["project-access-groups", projectId] }),
  })

  function handleSubmit(e: FormEvent) {
    e.preventDefault()
    setError(null)
    addMember.mutate()
  }

  function handleGroupSubmit(e: FormEvent) {
    e.preventDefault()
    setError(null)
    linkGroup.mutate()
  }

  const columns: DataTableColumn<Member>[] = [
    { key: "name", header: t("pages.name"), render: (m) => m.name },
    { key: "email", header: t("pages.email"), render: (m) => m.email },
    {
      key: "role",
      header: t("pages.role"),
      render: (m) => (
        <Select value={m.role} onValueChange={(v) => updateRole.mutate({ userId: m.userId, role: v as ProjectRole })}>
          <SelectTrigger className="w-28" onClick={(e) => e.stopPropagation()}>
            <SelectValue>
              <RoleBadge role={m.role} />
            </SelectValue>
          </SelectTrigger>
          <SelectContent>
            {ROLES.map((r) => (
              <SelectItem key={r} value={r}>
                <RoleBadge role={r} />
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      ),
    },
    {
      key: "actions",
      header: "",
      render: (m) => (
        <Button variant="ghost" size="sm" onClick={(e) => { e.stopPropagation(); setPendingRemove(m) }}>
          {t("pages.remove")}
        </Button>
      ),
    },
  ]

  const selectableGroups = availableGroups.filter((group) => !projectGroups.some((grant) => grant.groupId === group.id))

  return (
    <div className="mx-auto flex w-full max-w-5xl flex-col gap-6 p-4 sm:p-6 lg:p-8">
      <div className="mb-6 flex items-start justify-between">
        <div>
          <h1 className="text-2xl font-semibold">{t("pages.membersTitle")}</h1>
          <p className="mt-1 text-sm text-muted-foreground">{t("pages.membersIntro")}</p>
        </div>
        <Dialog open={open} onOpenChange={setOpen}>
          <DialogTrigger asChild>
            <Button>{t("pages.addMember")}</Button>
          </DialogTrigger>
          <DialogContent>
            <DialogHeader>
              <DialogTitle>{t("pages.addMember")}</DialogTitle>
            </DialogHeader>
            <form className="flex flex-col gap-4" onSubmit={handleSubmit}>
              <div className="flex flex-col gap-2">
                <Label htmlFor="member-email">{t("pages.email")}</Label>
                <Input id="member-email" type="email" value={email} onChange={(e) => setEmail(e.target.value)} required />
                <p className="text-xs text-muted-foreground">{t("pages.userMustHaveAccount")}</p>
              </div>
              <div className="flex flex-col gap-2">
                <Label>{t("pages.role")}</Label>
                <Select value={role} onValueChange={(v) => setRole(v as ProjectRole)}>
                  <SelectTrigger>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    {ROLES.map((r) => (
                      <SelectItem key={r} value={r}>
                        <RoleBadge role={r} />
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>
              {error && <p className="text-sm text-destructive">{error}</p>}
              <DialogFooter>
                <Button type="submit" disabled={addMember.isPending}>
                  {t("common.create")}
                </Button>
              </DialogFooter>
            </form>
          </DialogContent>
        </Dialog>
      </div>
      <DataTable columns={columns} rows={members} rowKey={(m) => m.userId} emptyMessage={isLoading ? t("common.loading") : t("pages.noMembers")} />
      <section className="rounded-xl border bg-card">
        <div className="flex items-center justify-between gap-4 border-b p-5">
          <div><h2 className="font-semibold">{t("pages.accessGroupsTitle")}</h2><p className="mt-1 text-sm text-muted-foreground">{t("pages.accessGroupsIntro")}</p></div>
          <Dialog open={groupOpen} onOpenChange={setGroupOpen}>
            <DialogTrigger asChild><Button variant="outline">{t("pages.linkGroup")}</Button></DialogTrigger>
            <DialogContent><DialogHeader><DialogTitle>{t("pages.linkAccessGroup")}</DialogTitle></DialogHeader><form className="flex flex-col gap-4" onSubmit={handleGroupSubmit}>
              <div className="flex flex-col gap-2"><Label>{t("pages.group")}</Label><Select value={groupId} onValueChange={setGroupId}><SelectTrigger><SelectValue placeholder={t("pages.chooseGroup")} /></SelectTrigger><SelectContent>{selectableGroups.map((group) => <SelectItem key={group.id} value={group.id}>{group.name}</SelectItem>)}</SelectContent></Select></div>
              <div className="flex flex-col gap-2"><Label>{t("pages.projectRole")}</Label><Select value={groupRole} onValueChange={(value) => setGroupRole(value as ProjectRole)}><SelectTrigger><SelectValue /></SelectTrigger><SelectContent>{ROLES.filter((role) => role !== "owner").map((role) => <SelectItem key={role} value={role}><RoleBadge role={role} /></SelectItem>)}</SelectContent></Select></div>
              {error && <p className="text-sm text-destructive">{error}</p>}<DialogFooter><Button type="submit" disabled={!groupId || linkGroup.isPending}>{t("pages.linkGroup")}</Button></DialogFooter>
            </form></DialogContent>
          </Dialog>
        </div>
        <div className="divide-y">{projectGroups.map((group) => <div key={group.groupId} className="flex flex-col gap-3 p-5 sm:flex-row sm:items-center sm:justify-between"><div><p className="font-medium">{group.name}</p>{group.description && <p className="text-sm text-muted-foreground">{group.description}</p>}</div><div className="flex items-center gap-2"><Select value={group.role} onValueChange={(value) => updateGroupRole.mutate({ id: group.groupId, role: value as ProjectRole })}><SelectTrigger className="w-28"><SelectValue /></SelectTrigger><SelectContent>{ROLES.filter((role) => role !== "owner").map((role) => <SelectItem key={role} value={role}><RoleBadge role={role} /></SelectItem>)}</SelectContent></Select><Button variant="ghost" size="sm" onClick={() => unlinkGroup.mutate(group.groupId)}>{t("pages.unlink")}</Button></div></div>)}{projectGroups.length === 0 && <p className="p-5 text-sm text-muted-foreground">{isLoadingGroups ? t("pages.loadingGroups") : t("pages.noGroupsLinked")}</p>}</div>
      </section>
      <ConfirmDialog
        open={!!pendingRemove}
        onOpenChange={(v) => !v && setPendingRemove(null)}
        title={t("pages.removeMemberTitle", { name: pendingRemove?.name ?? "" })}
        description={t("pages.removeMemberDescription")}
        confirmLabel={t("pages.remove")}
        loading={removeMember.isPending}
        onConfirm={() => pendingRemove && removeMember.mutate(pendingRemove.userId)}
      />
    </div>
  )
}
