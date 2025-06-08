import { createLazyFileRoute, Outlet } from '@tanstack/react-router'

function SetupLayout() {
  return <Outlet />
}

export const Route = createLazyFileRoute('/setup')({
  component: SetupLayout,
}) 