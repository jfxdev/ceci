import { ChevronDown } from "lucide-react"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import {
  DropdownMenu,
  DropdownMenuCheckboxItem,
  DropdownMenuContent,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"
import { cn } from "@/lib/utils"

export interface MultiSelectOption {
  value: string
  label: string
}

interface MultiSelectProps {
  options: MultiSelectOption[]
  selected: string[]
  onChange: (selected: string[]) => void
  placeholder?: string
  className?: string
}

/**
 * Dropdown with checkbox items for picking multiple values from a fixed
 * option list. Not part of components/ui — a shared composition on top of
 * the shadcn DropdownMenu primitives, since shadcn ships no multi-select.
 */
export function MultiSelect({ options, selected, onChange, placeholder = "Select...", className }: MultiSelectProps) {
  function toggle(value: string) {
    onChange(selected.includes(value) ? selected.filter((v) => v !== value) : [...selected, value])
  }

  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button variant="outline" className={cn("w-full justify-between font-normal", className)}>
          <span className="flex flex-wrap items-center gap-1 overflow-hidden">
            {selected.length === 0 ? (
              <span className="text-muted-foreground">{placeholder}</span>
            ) : selected.length <= 3 ? (
              selected.map((value) => (
                <Badge key={value} variant="secondary" className="font-normal">
                  {options.find((o) => o.value === value)?.label ?? value}
                </Badge>
              ))
            ) : (
              <Badge variant="secondary" className="font-normal">
                {selected.length} selected
              </Badge>
            )}
          </span>
          <ChevronDown className="size-4 shrink-0 opacity-50" />
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent className="w-(--radix-dropdown-menu-trigger-width) min-w-56">
        {options.length === 0 && <div className="px-2 py-1.5 text-sm text-muted-foreground">No options</div>}
        {options.map((option) => (
          <DropdownMenuCheckboxItem
            key={option.value}
            checked={selected.includes(option.value)}
            onSelect={(e) => e.preventDefault()}
            onCheckedChange={() => toggle(option.value)}
          >
            {option.label}
          </DropdownMenuCheckboxItem>
        ))}
      </DropdownMenuContent>
    </DropdownMenu>
  )
}
