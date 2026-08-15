import { useMemo } from "react"
import { Link, useParams } from "react-router-dom"
import { useQuery } from "@tanstack/react-query"
import { Bar, BarChart, CartesianGrid, XAxis } from "recharts"
import { Badge } from "@/components/ui/badge"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { ChartContainer, ChartTooltip, ChartTooltipContent, type ChartConfig } from "@/components/ui/chart"
import { DataTable, type DataTableColumn } from "@/components/shared/data-table"
import { api } from "@/lib/api"
import { useEnvironment } from "@/lib/environment"

interface Flag {
  key: string
  name: string
  flagType: string
  enabled: boolean
}

interface Parameter {
  key: string
  value: string
  version: number
}

interface Member {
  userId: string
  email: string
  role: string
}

const chartConfig: ChartConfig = {
  count: { label: "Flags", color: "var(--primary)" },
}

export function ProjectOverviewPage() {
  const { projectId } = useParams<{ projectId: string }>()
  const { envKey, envPath } = useEnvironment()

  const { data: flags = [], isLoading: flagsLoading } = useQuery({
    queryKey: ["flags", projectId, envKey],
    queryFn: () => api.get<Flag[]>(envPath("flags")),
    enabled: !!projectId && !!envKey,
  })

  const { data: parameters = [] } = useQuery({
    queryKey: ["parameters", projectId, envKey, ""],
    queryFn: () => api.get<Parameter[]>(`${envPath("parameters")}?prefix=`),
    enabled: !!projectId && !!envKey,
  })

  const { data: members = [] } = useQuery({
    queryKey: ["members", projectId],
    queryFn: () => api.get<Member[]>(`/projects/${projectId}/members`),
    enabled: !!projectId,
  })

  const enabledCount = flags.filter((f) => f.enabled).length

  const typeBreakdown = useMemo(() => {
    const counts = new Map<string, number>()
    for (const f of flags) counts.set(f.flagType, (counts.get(f.flagType) ?? 0) + 1)
    return Array.from(counts.entries()).map(([flagType, count]) => ({ flagType, count }))
  }, [flags])

  const recentFlags = flags.slice(0, 5)

  const columns: DataTableColumn<Flag>[] = [
    {
      key: "name",
      header: "Name",
      render: (f) => (
        <Link to={`/projects/${projectId}/flags/${f.key}`} className="font-medium hover:underline">
          {f.name}
        </Link>
      ),
    },
    { key: "key", header: "Key", render: (f) => <code className="text-xs">{f.key}</code> },
    { key: "flagType", header: "Type", render: (f) => f.flagType },
    {
      key: "enabled",
      header: "Status",
      render: (f) => <Badge variant={f.enabled ? "default" : "outline"}>{f.enabled ? "Enabled" : "Disabled"}</Badge>,
    },
  ]

  return (
    <div className="flex flex-col gap-4 p-4 lg:gap-6 lg:p-6">
      <div className="grid grid-cols-1 gap-4 *:data-[slot=card]:bg-gradient-to-t *:data-[slot=card]:from-primary/5 *:data-[slot=card]:to-card *:data-[slot=card]:shadow-xs sm:grid-cols-2 xl:grid-cols-4 dark:*:data-[slot=card]:bg-card">
        <Card>
          <CardHeader>
            <CardDescription>Flags</CardDescription>
            <CardTitle className="text-2xl font-semibold tabular-nums">{flagsLoading ? "…" : flags.length}</CardTitle>
          </CardHeader>
        </Card>
        <Card>
          <CardHeader>
            <CardDescription>Enabled flags</CardDescription>
            <CardTitle className="text-2xl font-semibold tabular-nums">{flagsLoading ? "…" : enabledCount}</CardTitle>
          </CardHeader>
        </Card>
        <Card>
          <CardHeader>
            <CardDescription>Parameters</CardDescription>
            <CardTitle className="text-2xl font-semibold tabular-nums">{parameters.length}</CardTitle>
          </CardHeader>
        </Card>
        <Card>
          <CardHeader>
            <CardDescription>Members</CardDescription>
            <CardTitle className="text-2xl font-semibold tabular-nums">{members.length}</CardTitle>
          </CardHeader>
        </Card>
      </div>

      <Card>
        <CardHeader>
          <CardTitle>Flags by type</CardTitle>
          <CardDescription>Breakdown of flag types in this project</CardDescription>
        </CardHeader>
        <CardContent>
          <ChartContainer config={chartConfig} className="aspect-auto h-[220px] w-full">
            <BarChart data={typeBreakdown}>
              <CartesianGrid vertical={false} />
              <XAxis dataKey="flagType" tickLine={false} axisLine={false} tickMargin={8} />
              <ChartTooltip content={<ChartTooltipContent />} />
              <Bar dataKey="count" fill="var(--color-count)" radius={4} />
            </BarChart>
          </ChartContainer>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>Recent flags</CardTitle>
          <CardDescription>Latest flags in this project</CardDescription>
        </CardHeader>
        <CardContent>
          <DataTable columns={columns} rows={recentFlags} rowKey={(f) => f.key} emptyMessage="No flags yet" />
        </CardContent>
      </Card>
    </div>
  )
}
