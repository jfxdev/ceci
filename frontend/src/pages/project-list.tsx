import { useState, type FormEvent } from "react"
import { useNavigate } from "react-router-dom"
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Dialog, DialogContent, DialogFooter, DialogHeader, DialogTitle, DialogTrigger } from "@/components/ui/dialog"
import { DataTable, type DataTableColumn } from "@/components/shared/data-table"
import { api } from "@/lib/api"

interface Project {
  id: string
  name: string
  slug: string
}

const columns: DataTableColumn<Project>[] = [
  { key: "name", header: "Name", render: (p) => p.name },
  { key: "slug", header: "Slug", render: (p) => p.slug },
]

export function ProjectListPage() {
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const { data: projects = [], isLoading } = useQuery({
    queryKey: ["projects"],
    queryFn: () => api.get<Project[]>("/projects"),
  })

  const [open, setOpen] = useState(false)
  const [name, setName] = useState("")
  const [slug, setSlug] = useState("")

  const createProject = useMutation({
    mutationFn: () => api.post<Project>("/projects", { name, slug }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["projects"] })
      setOpen(false)
      setName("")
      setSlug("")
    },
  })

  function handleSubmit(e: FormEvent) {
    e.preventDefault()
    createProject.mutate()
  }

  return (
    <div className="p-8">
      <div className="mb-6 flex items-center justify-between">
        <h1 className="text-2xl font-semibold">Projects</h1>
        <Dialog open={open} onOpenChange={setOpen}>
          <DialogTrigger asChild>
            <Button>New project</Button>
          </DialogTrigger>
          <DialogContent>
            <DialogHeader>
              <DialogTitle>Create project</DialogTitle>
            </DialogHeader>
            <form className="flex flex-col gap-4" onSubmit={handleSubmit}>
              <div className="flex flex-col gap-2">
                <Label htmlFor="name">Name</Label>
                <Input id="name" value={name} onChange={(e) => setName(e.target.value)} required />
              </div>
              <div className="flex flex-col gap-2">
                <Label htmlFor="slug">Slug</Label>
                <Input id="slug" value={slug} onChange={(e) => setSlug(e.target.value)} required />
              </div>
              <DialogFooter>
                <Button type="submit" disabled={createProject.isPending}>
                  Create
                </Button>
              </DialogFooter>
            </form>
          </DialogContent>
        </Dialog>
      </div>
      <DataTable
        columns={columns}
        rows={projects}
        rowKey={(p) => p.id}
        emptyMessage={isLoading ? "Loading..." : "No projects yet"}
        onRowClick={(p) => navigate(`/projects/${p.id}/flags`)}
      />
    </div>
  )
}
