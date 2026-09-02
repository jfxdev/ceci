import { useId, useMemo } from "react"

import { Input } from "@/components/ui/input"
import { useTranslation } from "react-i18next"

interface AttributeInputProps {
  attributes: string[]
  value: string
  onChange: (value: string) => void
}

function AttributeInput({ attributes, value, onChange }: AttributeInputProps) {
	const { t } = useTranslation()
  const listId = useId()
  const options = useMemo(() => [...new Set(attributes)].sort(), [attributes])
  const isNewAttribute = Boolean(value.trim()) && !options.includes(value.trim())

  return (
    <div className="relative w-64 shrink-0">
      <Input
	        aria-label={t("pages.attribute")}
        list={listId}
	        placeholder={t("pages.attributePlaceholder")}
        value={value}
        onChange={(event) => onChange(event.target.value)}
        className={isNewAttribute ? "pr-14" : undefined}
      />
      <datalist id={listId}>
        {options.map((attribute) => <option key={attribute} value={attribute} />)}
      </datalist>
      {isNewAttribute && (
        <span className="pointer-events-none absolute inset-y-0 right-3 flex items-center text-xs font-medium text-primary">
	          {t("pages.newBadge")}
        </span>
      )}
    </div>
  )
}

export { AttributeInput, type AttributeInputProps }
