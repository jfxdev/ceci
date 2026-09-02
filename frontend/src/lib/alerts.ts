import { toast } from "sonner"
import i18n from "@/lib/i18n"
import { ApiError } from "@/lib/api"

type AlertMessage = Parameters<typeof toast.success>[0]
type AlertOptions = Parameters<typeof toast.success>[1]

function messageFrom(error: unknown, fallback: string): string {
  if (error instanceof ApiError) {
    if (error.code && i18n.exists(`errors.${error.code}`)) return i18n.t(`errors.${error.code}`)
    if (error.status === 401) return i18n.t("errors.unauthorized")
  }
  return error instanceof Error && error.message ? error.message : fallback
}

/**
 * Application-wide notification API.
 *
 * The Toaster is mounted once in App, so any component, hook, or service can
 * import this module and publish a consistent shadcn/Sonner notification.
 */
export const alerts = {
  success: (message: AlertMessage, options?: AlertOptions) => toast.success(message, options),
  error: (message: AlertMessage, options?: AlertOptions) => toast.error(message, options),
  warning: (message: AlertMessage, options?: AlertOptions) => toast.warning(message, options),
  info: (message: AlertMessage, options?: AlertOptions) => toast.info(message, options),
  loading: (message: AlertMessage, options?: AlertOptions) => toast.loading(message, options),
  dismiss: (toastId?: string | number) => toast.dismiss(toastId),
  errorFrom: (error: unknown, fallback = i18n.t("errors.generic")) => toast.error(messageFrom(error, fallback)),
}

export { messageFrom as alertMessageFromError }
