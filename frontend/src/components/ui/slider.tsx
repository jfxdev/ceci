import * as React from "react"
import { cn } from "@/lib/utils"

interface SliderProps extends Omit<React.InputHTMLAttributes<HTMLInputElement>, "value" | "onChange" | "type"> {
  value: number
  onValueChange: (value: number) => void
  min?: number
  max?: number
  step?: number
}

/**
 * Minimal range slider built on a native <input type="range"> — no extra
 * dependency, and it responds to real pointer events (unlike Radix, which the
 * agent browser tooling can't drive; see CLAUDE.md).
 */
export function Slider({ value, onValueChange, min = 0, max = 100, step = 1, className, ...props }: SliderProps) {
  return (
    <input
      type="range"
      min={min}
      max={max}
      step={step}
      value={value}
      onChange={(e) => onValueChange(Number(e.target.value))}
      className={cn(
        "h-2 w-full cursor-pointer appearance-none rounded-full bg-secondary accent-primary",
        className,
      )}
      {...props}
    />
  )
}
