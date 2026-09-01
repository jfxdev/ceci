import * as React from "react"
import { Plus } from "lucide-react"

import { Button } from "@/components/ui/button"
import { cn } from "@/lib/utils"

type ActionButtonProps = React.ComponentProps<typeof Button> & {
  icon?: React.ReactNode
}

function ActionButton({ children, className, icon = <Plus />, variant, ...props }: ActionButtonProps) {
  return (
    <Button
      variant={variant}
      className={cn(
        variant === "outline" && "border-primary text-primary hover:bg-primary hover:text-primary-foreground",
        className,
      )}
      {...props}
    >
      <span data-slot="action-button-icon" aria-hidden="true">
        {icon}
      </span>
      {children}
    </Button>
  )
}

export { ActionButton, type ActionButtonProps }
