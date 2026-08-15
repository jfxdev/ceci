import { useEffect, useState, type FormEvent } from "react"
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { Plus, Trash2, Users } from "lucide-react"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Dialog, DialogContent, DialogFooter, DialogHeader, DialogTitle, DialogTrigger } from "@/components/ui/dialog"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { ConfirmDialog } from "@/components/shared/confirm-dialog"
import { api, ApiError } from "@/lib/api"

interface AccessGroup { id: string; name: string; description: string }
interface Member { userId: string; email: string; name: string }
interface OIDCMapping { externalGroup: string }
interface OIDCConfig { enabled: boolean; issuerUrl: string; clientId: string; redirectUrl: string; jitEnabled: boolean; hasClientSecret: boolean }

export function AccessGroupsPage() {
  const queryClient = useQueryClient()
  const [createOpen, setCreateOpen] = useState(false)
  const [membersGroup, setMembersGroup] = useState<AccessGroup | null>(null)
  const [deleteGroup, setDeleteGroup] = useState<AccessGroup | null>(null)
  const [name, setName] = useState("")
  const [description, setDescription] = useState("")
  const [email, setEmail] = useState("")
  const [externalGroup, setExternalGroup] = useState("")
  const [error, setError] = useState<string | null>(null)
  const [oidcConfig, setOIDCConfig] = useState({ enabled: false, issuerUrl: "", clientId: "", clientSecret: "", redirectUrl: "", jitEnabled: true })
  const { data: groups = [], isLoading } = useQuery({ queryKey: ["access-groups"], queryFn: () => api.get<AccessGroup[]>("/admin/access-groups") })
  const { data: savedOIDC } = useQuery({ queryKey: ["oidc-config"], queryFn: () => api.get<OIDCConfig>("/admin/oidc") })
  useEffect(() => { if (savedOIDC) setOIDCConfig((current) => ({ ...current, enabled: savedOIDC.enabled, issuerUrl: savedOIDC.issuerUrl, clientId: savedOIDC.clientId, redirectUrl: savedOIDC.redirectUrl, jitEnabled: savedOIDC.jitEnabled })) }, [savedOIDC])
  const { data: members = [] } = useQuery({
    queryKey: ["access-group-members", membersGroup?.id],
    queryFn: () => api.get<Member[]>(`/admin/access-groups/${membersGroup?.id}/members`),
    enabled: !!membersGroup,
  })
  const { data: oidcMappings = [] } = useQuery({
    queryKey: ["access-group-oidc-mappings", membersGroup?.id],
    queryFn: () => api.get<OIDCMapping[]>(`/admin/access-groups/${membersGroup?.id}/oidc-mappings`),
    enabled: !!membersGroup,
  })
  const create = useMutation({
    mutationFn: () => api.post<AccessGroup>("/admin/access-groups", { name, description }),
    onSuccess: () => { queryClient.invalidateQueries({ queryKey: ["access-groups"] }); setName(""); setDescription(""); setCreateOpen(false) },
    onError: (err) => setError(err instanceof ApiError ? err.message : "Failed to create group"),
  })
  const remove = useMutation({
    mutationFn: (group: AccessGroup) => api.delete(`/admin/access-groups/${group.id}`),
    onSuccess: () => { queryClient.invalidateQueries({ queryKey: ["access-groups"] }); setDeleteGroup(null) },
  })
  const addMember = useMutation({
    mutationFn: () => api.post(`/admin/access-groups/${membersGroup?.id}/members`, { email }),
    onSuccess: () => { queryClient.invalidateQueries({ queryKey: ["access-group-members", membersGroup?.id] }); setEmail("") },
    onError: (err) => setError(err instanceof ApiError ? err.message : "Failed to add member"),
  })
  const removeMember = useMutation({
    mutationFn: (userId: string) => api.delete(`/admin/access-groups/${membersGroup?.id}/members/${userId}`),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ["access-group-members", membersGroup?.id] }),
  })
  const addOIDCMapping = useMutation({
    mutationFn: () => api.post(`/admin/access-groups/${membersGroup?.id}/oidc-mappings`, { externalGroup }),
    onSuccess: () => { queryClient.invalidateQueries({ queryKey: ["access-group-oidc-mappings", membersGroup?.id] }); setExternalGroup("") },
    onError: (err) => setError(err instanceof ApiError ? err.message : "Failed to map OIDC group"),
  })
  const removeOIDCMapping = useMutation({
    mutationFn: (value: string) => api.delete(`/admin/access-groups/${membersGroup?.id}/oidc-mappings/${encodeURIComponent(value)}`),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ["access-group-oidc-mappings", membersGroup?.id] }),
  })
  const saveOIDC = useMutation({ mutationFn: () => api.put<OIDCConfig>("/admin/oidc", oidcConfig), onSuccess: () => queryClient.invalidateQueries({ queryKey: ["oidc-config"] }), onError: (err) => setError(err instanceof ApiError ? err.message : "Failed to save OIDC configuration") })

  function submitCreate(event: FormEvent) { event.preventDefault(); setError(null); create.mutate() }
  function submitMember(event: FormEvent) { event.preventDefault(); setError(null); addMember.mutate() }
  function submitOIDCMapping(event: FormEvent) { event.preventDefault(); setError(null); addOIDCMapping.mutate() }
  function submitOIDCConfig(event: FormEvent) { event.preventDefault(); setError(null); saveOIDC.mutate() }

  return <div className="mx-auto flex w-full max-w-5xl flex-col gap-6 p-4 sm:p-6 lg:p-8">
    <div className="flex flex-col gap-4 sm:flex-row sm:items-end sm:justify-between">
      <div><p className="text-sm font-medium text-primary">Administration</p><h1 className="mt-1 text-3xl font-semibold tracking-tight">Access groups</h1><p className="mt-2 max-w-2xl text-sm leading-6 text-muted-foreground">Reuse teams across projects. Members can be managed here today and mapped to identity-provider groups when SSO is enabled.</p></div>
      <Dialog open={createOpen} onOpenChange={setCreateOpen}><DialogTrigger asChild><Button><Plus /> New group</Button></DialogTrigger><DialogContent><DialogHeader><DialogTitle>Create access group</DialogTitle></DialogHeader><form className="flex flex-col gap-4" onSubmit={submitCreate}>
        <div className="flex flex-col gap-2"><Label htmlFor="group-name">Name</Label><Input id="group-name" value={name} onChange={(event) => setName(event.target.value)} placeholder="Payments team" required /></div>
        <div className="flex flex-col gap-2"><Label htmlFor="group-description">Description</Label><Input id="group-description" value={description} onChange={(event) => setDescription(event.target.value)} placeholder="Maintains payment services" /></div>
        {error && <p className="text-sm text-destructive">{error}</p>}<DialogFooter><Button type="submit" disabled={create.isPending}>{create.isPending ? "Creating…" : "Create group"}</Button></DialogFooter>
      </form></DialogContent></Dialog>
    </div>
    <Card><CardHeader><CardTitle>OIDC single sign-on</CardTitle><CardDescription>Configure Microsoft Entra ID or Authentik. The client secret is encrypted before persistence.</CardDescription></CardHeader><CardContent><form className="grid gap-3 sm:grid-cols-2" onSubmit={submitOIDCConfig}><label className="flex items-center gap-2 text-sm sm:col-span-2"><input type="checkbox" checked={oidcConfig.enabled} onChange={(e) => setOIDCConfig({ ...oidcConfig, enabled: e.target.checked })} /> Enable OIDC</label><Input value={oidcConfig.issuerUrl} onChange={(e) => setOIDCConfig({ ...oidcConfig, issuerUrl: e.target.value })} placeholder="Issuer URL" disabled={!oidcConfig.enabled} /><Input value={oidcConfig.clientId} onChange={(e) => setOIDCConfig({ ...oidcConfig, clientId: e.target.value })} placeholder="Client ID" disabled={!oidcConfig.enabled} /><Input type="password" value={oidcConfig.clientSecret} onChange={(e) => setOIDCConfig({ ...oidcConfig, clientSecret: e.target.value })} placeholder={savedOIDC?.hasClientSecret ? "Client secret (unchanged)" : "Client secret"} disabled={!oidcConfig.enabled} /><Input value={oidcConfig.redirectUrl} onChange={(e) => setOIDCConfig({ ...oidcConfig, redirectUrl: e.target.value })} placeholder="Redirect URL" disabled={!oidcConfig.enabled} /><label className="flex items-center gap-2 text-sm"><input type="checkbox" checked={oidcConfig.jitEnabled} onChange={(e) => setOIDCConfig({ ...oidcConfig, jitEnabled: e.target.checked })} disabled={!oidcConfig.enabled} /> Create users on first login</label><div className="flex justify-end"><Button type="submit" disabled={saveOIDC.isPending}>{saveOIDC.isPending ? "Saving…" : "Save OIDC"}</Button></div></form></CardContent></Card>
    {isLoading ? <p className="text-sm text-muted-foreground">Loading groups…</p> : groups.length === 0 ? <Card><CardContent className="py-12 text-center text-sm text-muted-foreground">No access groups yet.</CardContent></Card> : <div className="grid gap-4 sm:grid-cols-2">
      {groups.map((group) => <Card key={group.id}><CardHeader><CardTitle className="flex items-center gap-2 text-lg"><Users className="size-5" />{group.name}</CardTitle><CardDescription>{group.description || "No description"}</CardDescription></CardHeader><CardContent className="flex gap-2"><Button variant="outline" size="sm" onClick={() => { setError(null); setMembersGroup(group) }}>Manage members</Button><Button variant="ghost" size="icon-sm" aria-label={`Delete ${group.name}`} onClick={() => setDeleteGroup(group)}><Trash2 /></Button></CardContent></Card>)}
    </div>}
    <Dialog open={!!membersGroup} onOpenChange={(open) => !open && setMembersGroup(null)}><DialogContent><DialogHeader><DialogTitle>Members · {membersGroup?.name}</DialogTitle></DialogHeader><form className="flex gap-2" onSubmit={submitMember}><Input type="email" value={email} onChange={(event) => setEmail(event.target.value)} placeholder="person@company.com" required /><Button type="submit" disabled={addMember.isPending}>Add</Button></form>{error && <p className="text-sm text-destructive">{error}</p>}<div className="max-h-40 divide-y overflow-auto">{members.map((member) => <div key={member.userId} className="flex items-center justify-between py-3"><div><p className="text-sm font-medium">{member.name || member.email}</p><p className="text-xs text-muted-foreground">{member.email}</p></div><Button variant="ghost" size="sm" onClick={() => removeMember.mutate(member.userId)}>Remove</Button></div>)}{members.length === 0 && <p className="py-4 text-center text-sm text-muted-foreground">No members yet.</p>}</div><div className="border-t pt-4"><p className="mb-2 text-sm font-medium">OIDC group mapping</p><p className="mb-3 text-xs text-muted-foreground">Use an Entra group Object ID or the group value emitted by Authentik.</p><form className="flex gap-2" onSubmit={submitOIDCMapping}><Input value={externalGroup} onChange={(event) => setExternalGroup(event.target.value)} placeholder="External group ID or name" required /><Button type="submit" disabled={addOIDCMapping.isPending}>Map</Button></form><div className="mt-3 divide-y">{oidcMappings.map((mapping) => <div key={mapping.externalGroup} className="flex items-center justify-between py-2"><code className="max-w-72 truncate text-xs">{mapping.externalGroup}</code><Button variant="ghost" size="sm" onClick={() => removeOIDCMapping.mutate(mapping.externalGroup)}>Remove</Button></div>)}{oidcMappings.length === 0 && <p className="py-2 text-xs text-muted-foreground">No OIDC groups mapped.</p>}</div></div></DialogContent></Dialog>
    <ConfirmDialog open={!!deleteGroup} onOpenChange={(open) => !open && setDeleteGroup(null)} title={`Delete “${deleteGroup?.name}”?`} description="This removes its memberships and all project associations." confirmLabel="Delete group" loading={remove.isPending} onConfirm={() => deleteGroup && remove.mutate(deleteGroup)} />
  </div>
}
