import { useEffect } from 'react'
import { createLazyFileRoute, useNavigate } from '@tanstack/react-router'

function IndexRedirect() {
  const navigate = useNavigate({ from: '/' })

  useEffect(() => {
    navigate({ to: '/dashboard', replace: true })
  }, [navigate])

  return null
}

export const Route = createLazyFileRoute('/')({
  component: IndexRedirect,
})
