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
import { useTranslation } from "react-i18next"

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
  const { t } = useTranslation()
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
      header: t("overview.name"),
      render: (f) => (
        <Link to={`/projects/${projectId}/flags/${f.key}`} className="font-medium hover:underline">
          {f.name}
        </Link>
      ),
    },
    { key: "key", header: t("overview.key"), render: (f) => <code className="text-xs">{f.key}</code> },
    { key: "flagType", header: t("overview.type"), render: (f) => f.flagType },
    {
      key: "enabled",
      header: t("overview.status"),
      render: (f) => <Badge variant={f.enabled ? "default" : "outline"}>{f.enabled ? t("overview.enabled") : t("overview.disabled")}</Badge>,
    },
  ]

  return (
    <div className="flex flex-col gap-4 p-4 lg:gap-6 lg:p-6">
      <div>
        <h1 className="text-2xl font-semibold">{t("nav.overview")}</h1>
        <p className="mt-1 text-sm text-muted-foreground">{t("overview.intro")}</p>
      </div>
      <div className="grid grid-cols-1 gap-4 *:data-[slot=card]:bg-gradient-to-t *:data-[slot=card]:from-primary/5 *:data-[slot=card]:to-card *:data-[slot=card]:shadow-xs sm:grid-cols-2 xl:grid-cols-4 dark:*:data-[slot=card]:bg-card">
        <Card>
          <CardHeader>
            <CardDescription>{t("nav.flags")}</CardDescription>
            <CardTitle className="text-2xl font-semibold tabular-nums">{flagsLoading ? "…" : flags.length}</CardTitle>
          </CardHeader>
        </Card>
        <Card>
          <CardHeader>
            <CardDescription>{t("overview.enabledFlags")}</CardDescription>
            <CardTitle className="text-2xl font-semibold tabular-nums">{flagsLoading ? "…" : enabledCount}</CardTitle>
          </CardHeader>
        </Card>
        <Card>
          <CardHeader>
            <CardDescription>{t("nav.parameters")}</CardDescription>
            <CardTitle className="text-2xl font-semibold tabular-nums">{parameters.length}</CardTitle>
          </CardHeader>
        </Card>
        <Card>
          <CardHeader>
            <CardDescription>{t("nav.members")}</CardDescription>
            <CardTitle className="text-2xl font-semibold tabular-nums">{members.length}</CardTitle>
          </CardHeader>
        </Card>
      </div>

      <Card>
        <CardHeader>
          <CardTitle>{t("overview.flagsByType")}</CardTitle>
          <CardDescription>{t("overview.breakdown")}</CardDescription>
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
          <CardTitle>{t("overview.recentFlags")}</CardTitle>
          <CardDescription>{t("overview.recentDescription")}</CardDescription>
        </CardHeader>
        <CardContent>
          <DataTable columns={columns} rows={recentFlags} rowKey={(f) => f.key} emptyMessage={t("overview.noFlagsYet")} />
        </CardContent>
      </Card>
    </div>
  )
}
