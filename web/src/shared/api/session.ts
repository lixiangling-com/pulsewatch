type Listener = () => void

let accessToken: string | null = null
const listeners = new Set<Listener>()

function notify() {
  listeners.forEach((listener) => listener())
}

export const session = {
  getAccessToken: () => accessToken,
  setAccessToken: (token: string | null) => {
    accessToken = token
    notify()
  },
  clear: () => {
    accessToken = null
    notify()
  },
  subscribe: (listener: Listener) => {
    listeners.add(listener)
    return () => listeners.delete(listener)
  },
}
