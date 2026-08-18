import { useEffect, useMemo, useState, type FormEvent } from "react"
import { useNavigate } from "react-router-dom"
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { FolderKanban, Grid2X2, List, Plus, Search, Settings2, Sparkles } from "lucide-react"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Dialog, DialogContent, DialogFooter, DialogHeader, DialogTitle, DialogTrigger } from "@/components/ui/dialog"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Checkbox } from "@/components/ui/checkbox"
import { DataTable, type DataTableColumn } from "@/components/shared/data-table"
import { api } from "@/lib/api"
import { cn } from "@/lib/utils"

interface Project {
  id: string
  name: string
  slug: string
}

interface EnvironmentTemplate {
  id: string
  key: string
  name: string
  isRequired: boolean
}

type ViewMode = "grid" | "list"

const projectColors = [
  "bg-sky-500/10 text-sky-700 dark:text-sky-300",
  "bg-violet-500/10 text-violet-700 dark:text-violet-300",
  "bg-emerald-500/10 text-emerald-700 dark:text-emerald-300",
  "bg-amber-500/10 text-amber-700 dark:text-amber-300",
  "bg-rose-500/10 text-rose-700 dark:text-rose-300",
]

function projectColor(name: string) {
  const value = Array.from(name).reduce((total, character) => total + character.charCodeAt(0), 0)
  return projectColors[value % projectColors.length]
}

function slugFromName(value: string) {
  return value
    .toLowerCase()
    .normalize("NFD")
    .replace(/[\u0300-\u036f]/g, "")
    .replace(/[^a-z0-9]+/g, "-")
    .replace(/(^-|-$)/g, "")
}

export function ProjectListPage() {
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const { data: projects = [], isLoading } = useQuery({
    queryKey: ["projects"],
    queryFn: () => api.get<Project[]>("/projects"),
  })
  const { data: environmentTemplates = [] } = useQuery({
    queryKey: ["admin", "environment-templates"],
    queryFn: () => api.get<EnvironmentTemplate[]>("/admin/environment-templates"),
  })

  const [open, setOpen] = useState(false)
  const [name, setName] = useState("")
  const [slug, setSlug] = useState("")
  const [hasCustomSlug, setHasCustomSlug] = useState(false)
  const [environmentTemplateKeys, setEnvironmentTemplateKeys] = useState<string[]>([])
  const [query, setQuery] = useState("")
  const [viewMode, setViewMode] = useState<ViewMode>("grid")

  const matchingProjects = useMemo(() => {
    const normalizedQuery = query.trim().toLocaleLowerCase()
    if (!normalizedQuery) return projects
    return projects.filter((project) =>
      [project.name, project.slug].some((value) => value.toLocaleLowerCase().includes(normalizedQuery))
    )
  }, [projects, query])

  const createProject = useMutation({
    mutationFn: () => api.post<Project>("/projects", { name, slug, environmentTemplateKeys }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["projects"] })
      resetProjectForm()
      setOpen(false)
    },
  })

  function resetProjectForm() {
    setName("")
    setSlug("")
    setHasCustomSlug(false)
    setEnvironmentTemplateKeys(environmentTemplates.filter((template) => template.isRequired).map((template) => template.key))
  }

  useEffect(() => {
    if (!open) setEnvironmentTemplateKeys(environmentTemplates.filter((template) => template.isRequired).map((template) => template.key))
  }, [environmentTemplates, open])

  function handleProjectNameChange(value: string) {
    setName(value)
    if (!hasCustomSlug) setSlug(slugFromName(value))
  }

  function handleDialogOpenChange(nextOpen: boolean) {
    setOpen(nextOpen)
    if (!nextOpen) resetProjectForm()
  }

  function handleSubmit(e: FormEvent) {
    e.preventDefault()
    createProject.mutate()
  }

  const columns: DataTableColumn<Project>[] = [
    {
      key: "name",
      header: "Project",
      render: (project) => (
        <div className="flex items-center gap-3">
          <ProjectMark project={project} size="small" />
          <span className="font-medium">{project.name}</span>
        </div>
      ),
    },
    {
      key: "slug",
      header: "Project key",
      render: (project) => <code className="rounded bg-muted px-2 py-1 text-xs text-muted-foreground">{project.slug}</code>,
    },
    {
      key: "open",
      header: "",
      className: "w-28 text-right",
      render: (project) => (
        <Button
          variant="ghost"
          size="sm"
          onClick={(event) => {
            event.stopPropagation()
            navigate(`/projects/${project.id}/overview`)
          }}
        >
          Open
        </Button>
      ),
    },
  ]

  const hasProjects = projects.length > 0
  const isEmptySearch = hasProjects && matchingProjects.length === 0

  return (
    <div className="mx-auto flex w-full max-w-7xl flex-col gap-6 p-4 sm:p-6 lg:p-8">
      <section className="relative overflow-hidden rounded-2xl border bg-gradient-to-br from-primary/[0.07] via-background to-background p-6 sm:p-8">
        <div className="absolute -right-16 -top-16 size-48 rounded-full bg-primary/[0.07] blur-3xl" />
        <div className="relative flex flex-col gap-6 md:flex-row md:items-end md:justify-between">
          <div className="max-w-2xl">
            <div className="mb-3 flex items-center gap-2 text-sm font-medium text-muted-foreground">
              <Sparkles className="size-4 text-primary" />
              Your workspace
            </div>
            <h1 className="text-3xl font-semibold tracking-tight sm:text-4xl">Projects</h1>
            <p className="mt-3 text-sm leading-6 text-muted-foreground sm:text-base">
              Keep feature flags, parameters, and access organized in focused project spaces.
            </p>
          </div>
          <NewProjectDialog
            open={open}
            onOpenChange={handleDialogOpenChange}
            name={name}
            slug={slug}
            environmentTemplates={environmentTemplates}
            selectedEnvironmentTemplateKeys={environmentTemplateKeys}
            isSubmitting={createProject.isPending}
            onNameChange={handleProjectNameChange}
            onSlugChange={(value) => {
              setSlug(value)
              setHasCustomSlug(true)
            }}
            onEnvironmentTemplateChange={(templateKey, selected) => {
              setEnvironmentTemplateKeys((current) =>
                selected ? [...new Set([...current, templateKey])] : current.filter((key) => key !== templateKey)
              )
            }}
            onSubmit={handleSubmit}
          />
        </div>
      </section>

      {hasProjects ? (
        <>
          <div className="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
            <div>
              <h2 className="text-lg font-semibold">Your projects</h2>
              <p className="text-sm text-muted-foreground">
                {projects.length} {projects.length === 1 ? "project" : "projects"} available to you
              </p>
            </div>
            <div className="flex flex-col gap-2 sm:flex-row sm:items-center">
              <div className="relative sm:w-72">
                <Search className="pointer-events-none absolute left-3 top-1/2 size-4 -translate-y-1/2 text-muted-foreground" />
                <Input
                  value={query}
                  onChange={(event) => setQuery(event.target.value)}
                  placeholder="Search projects"
                  className="pl-9"
                  aria-label="Search projects"
                />
              </div>
              <div className="flex rounded-md border bg-background p-1" aria-label="Project view">
                <Button
                  variant={viewMode === "grid" ? "secondary" : "ghost"}
                  size="icon-sm"
                  onClick={() => setViewMode("grid")}
                  aria-label="Grid view"
                >
                  <Grid2X2 />
                </Button>
                <Button
                  variant={viewMode === "list" ? "secondary" : "ghost"}
                  size="icon-sm"
                  onClick={() => setViewMode("list")}
                  aria-label="List view"
                >
                  <List />
                </Button>
              </div>
            </div>
          </div>

          {isEmptySearch ? (
            <NoSearchResults onClear={() => setQuery("")} />
          ) : viewMode === "grid" ? (
            <div className="grid gap-4 sm:grid-cols-2 xl:grid-cols-3">
              {matchingProjects.map((project) => (
                <ProjectCard key={project.id} project={project} onOpen={() => navigate(`/projects/${project.id}/overview`)} />
              ))}
            </div>
          ) : (
            <Card className="gap-0 overflow-hidden py-0">
              <DataTable
                columns={columns}
                rows={matchingProjects}
                rowKey={(project) => project.id}
                onRowClick={(project) => navigate(`/projects/${project.id}/overview`)}
              />
            </Card>
          )}
        </>
      ) : (
        <EmptyProjects isLoading={isLoading} onCreate={() => setOpen(true)} />
      )}
    </div>
  )
}

function NewProjectDialog({
  open,
  onOpenChange,
  name,
  slug,
  environmentTemplates,
  selectedEnvironmentTemplateKeys,
  isSubmitting,
  onNameChange,
  onSlugChange,
  onEnvironmentTemplateChange,
  onSubmit,
}: {
  open: boolean
  onOpenChange: (open: boolean) => void
  name: string
  slug: string
  environmentTemplates: EnvironmentTemplate[]
  selectedEnvironmentTemplateKeys: string[]
  isSubmitting: boolean
  onNameChange: (value: string) => void
  onSlugChange: (value: string) => void
  onEnvironmentTemplateChange: (templateKey: string, selected: boolean) => void
  onSubmit: (event: FormEvent) => void
}) {
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogTrigger asChild>
        <Button size="lg">
          <Plus />
          New project
        </Button>
      </DialogTrigger>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Create a project</DialogTitle>
          <p className="text-sm text-muted-foreground">A project is a shared home for your configuration and team.</p>
        </DialogHeader>
        <form className="flex flex-col gap-5" onSubmit={onSubmit}>
          <div className="flex flex-col gap-2">
            <Label htmlFor="name">Project name</Label>
            <Input id="name" value={name} onChange={(event) => onNameChange(event.target.value)} placeholder="e.g. Customer portal" required />
          </div>
          {environmentTemplates.length > 0 && (
            <div className="flex flex-col gap-2">
              <Label>Project environments</Label>
              <div className="rounded-lg border">
                {environmentTemplates.map((template) => {
                  const checked = template.isRequired || selectedEnvironmentTemplateKeys.includes(template.key)
                  return (
                    <label key={template.id} className="flex cursor-pointer items-center gap-3 border-b px-3 py-3 last:border-b-0">
                      <Checkbox
                        checked={checked}
                        disabled={template.isRequired}
                        onCheckedChange={(value) => onEnvironmentTemplateChange(template.key, value === true)}
                      />
                      <span className="flex-1">
                        <span className="block text-sm font-medium">{template.name}</span>
                        <code className="text-xs text-muted-foreground">{template.key}</code>
                      </span>
                      {template.isRequired && <span className="text-xs font-medium text-muted-foreground">Required</span>}
                    </label>
                  )
                })}
              </div>
              <p className="text-xs text-muted-foreground">Required environments are always included in new projects.</p>
            </div>
          )}
          <div className="flex flex-col gap-2">
            <Label htmlFor="slug">Project key</Label>
            <Input id="slug" value={slug} onChange={(event) => onSlugChange(event.target.value)} placeholder="customer-portal" required />
            <p className="text-xs text-muted-foreground">Used to identify this project in the URL and API.</p>
          </div>
          <DialogFooter>
            <Button type="submit" disabled={isSubmitting || !name.trim() || !slug.trim()}>
              {isSubmitting ? "Creating…" : "Create project"}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}

function ProjectMark({ project, size = "default" }: { project: Project; size?: "default" | "small" }) {
  return (
    <div
      className={cn(
        "flex shrink-0 items-center justify-center rounded-lg",
        size === "small" ? "size-8 rounded-md" : "size-11",
        projectColor(project.name)
      )}
    >
      <FolderKanban className={size === "small" ? "size-4" : "size-5"} />
    </div>
  )
}

function ProjectCard({ project, onOpen }: { project: Project; onOpen: () => void }) {
  return (
    <Card className="group gap-5 py-5 transition-shadow hover:shadow-md">
      <CardHeader className="px-5">
        <ProjectMark project={project} />
        <div className="mt-3">
          <CardTitle className="text-base">{project.name}</CardTitle>
          <CardDescription className="mt-1 font-mono text-xs">{project.slug}</CardDescription>
        </div>
      </CardHeader>
      <CardContent className="px-5">
        <div className="flex items-center justify-between border-t pt-4">
          <span className="text-xs text-muted-foreground">Configuration workspace</span>
          <Button variant="ghost" size="sm" onClick={onOpen}>
            Open
          </Button>
        </div>
      </CardContent>
    </Card>
  )
}

function EmptyProjects({ isLoading, onCreate }: { isLoading: boolean; onCreate: () => void }) {
  if (isLoading) {
    return <Card className="min-h-80 animate-pulse bg-muted/30" />
  }

  return (
    <Card className="overflow-hidden py-0">
      <div className="grid lg:grid-cols-[1.25fr_0.75fr]">
        <div className="p-6 sm:p-8">
          <div className="mb-5 flex size-12 items-center justify-center rounded-xl bg-primary text-primary-foreground">
            <FolderKanban className="size-6" />
          </div>
          <h2 className="text-xl font-semibold">Create your first project</h2>
          <p className="mt-2 max-w-lg text-sm leading-6 text-muted-foreground">
            Projects keep each product’s flags, runtime parameters, environments, and member access in one place.
          </p>
          <Button className="mt-6" onClick={onCreate}>
            <Plus />
            Create project
          </Button>
        </div>
        <div className="border-t bg-muted/40 p-6 lg:border-l lg:border-t-0 sm:p-8">
          <p className="text-sm font-medium">What you can do next</p>
          <div className="mt-5 space-y-4">
            <EmptyStep icon={<Settings2 className="size-4" />} title="Organize configuration" description="Keep flags and parameters together." />
            <EmptyStep icon={<Sparkles className="size-4" />} title="Ship with confidence" description="Control rollouts per environment." />
            <EmptyStep icon={<FolderKanban className="size-4" />} title="Invite your team" description="Give teammates the right level of access." />
          </div>
        </div>
      </div>
    </Card>
  )
}

function EmptyStep({ icon, title, description }: { icon: React.ReactNode; title: string; description: string }) {
  return (
    <div className="flex gap-3">
      <div className="mt-0.5 flex size-7 shrink-0 items-center justify-center rounded-md bg-background text-muted-foreground shadow-xs">{icon}</div>
      <div>
        <p className="text-sm font-medium">{title}</p>
        <p className="mt-0.5 text-xs leading-5 text-muted-foreground">{description}</p>
      </div>
    </div>
  )
}

function NoSearchResults({ onClear }: { onClear: () => void }) {
  return (
    <Card className="items-center gap-3 py-12 text-center">
      <Search className="size-5 text-muted-foreground" />
      <div>
        <CardTitle className="text-base">No projects found</CardTitle>
        <CardDescription className="mt-1">Try a different project name or key.</CardDescription>
      </div>
      <Button variant="outline" size="sm" onClick={onClear}>
        Clear search
      </Button>
    </Card>
  )
}
