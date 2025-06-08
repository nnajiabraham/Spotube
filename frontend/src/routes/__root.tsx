import { createRootRoute, Outlet, redirect } from '@tanstack/react-router'

// Check if setup is required by calling the backend
async function checkSetupStatus() {
  try {
    const response = await fetch('/api/setup/status')
    if (!response.ok) {
      throw new Error('Failed to check setup status')
    }
    const data = await response.json()
    return data.required
  } catch (error) {
    console.error('Error checking setup status:', error)
    // Default to requiring setup if we can't check
    return true
  }
}

export const Route = createRootRoute({
  component: () => <Outlet />,
  beforeLoad: async () => {
    const setupRequired = await checkSetupStatus()
    
    // If setup is required and we're not already on setup routes, redirect to setup
    if (setupRequired && !window.location.pathname.startsWith('/setup')) {
      throw redirect({ to: '/setup' })
    }
    
    // If setup is not required and we're on setup routes, redirect to dashboard
    if (!setupRequired && window.location.pathname.startsWith('/setup')) {
      throw redirect({ to: '/dashboard' })
    }
  },
}) 