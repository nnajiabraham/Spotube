import { createLazyFileRoute, useNavigate } from '@tanstack/react-router'
import { useEffect } from 'react'
import { CheckCircle2 } from 'lucide-react'
import { Button, Card } from '../../components/ui'

function SetupSuccess() {
  const navigate = useNavigate()

  useEffect(() => {
    const timer = setTimeout(() => navigate({ to: '/dashboard' }), 3000)
    return () => clearTimeout(timer)
  }, [navigate])

  return (
    <main className="mx-auto flex min-h-screen max-w-md flex-col justify-center px-4 py-12">
      <Card className="p-8 text-center">
        <CheckCircle2 className="mx-auto h-10 w-10 text-spotify-600" />
        <h1 className="mt-4 text-xl font-semibold text-ink">Credentials saved</h1>
        <p className="mt-1 text-sm text-gray-500">Next, connect Spotify and YouTube on the dashboard. Taking you there…</p>
        <Button variant="primary" className="mt-6 w-full" onClick={() => navigate({ to: '/dashboard' })}>
          Continue to dashboard
        </Button>
      </Card>
    </main>
  )
}

export const Route = createLazyFileRoute('/setup/success')({
  component: SetupSuccess,
})
