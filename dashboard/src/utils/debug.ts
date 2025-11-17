// Debug utility for development
export const debug = {
  log: (message: string, data?: any) => {
    if (import.meta.env.DEV) {
      console.log(`[KSAM Debug] ${message}`, data || '')
    }
  },
  error: (message: string, error?: any) => {
    if (import.meta.env.DEV) {
      console.error(`[KSAM Error] ${message}`, error || '')
    }
  },
}

