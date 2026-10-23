import { createLazyFileRoute, useNavigate } from '@tanstack/react-router'
import { useForm, type UseFormRegisterReturn } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { z } from 'zod'
import { useState } from 'react'
import { KeyRound } from 'lucide-react'
import { setupAPI } from '../../lib/api'
import { inputClass } from '../../lib/format'
import { Button, Card, Notice, ServiceIcon, SpotubeMark } from '../../components/ui'

const SetupSchema = z.object({
  spotifyId: z.string().min(1, 'Spotify Client ID is required'),
  spotifySecret: z.string().min(1, 'Spotify Client Secret is required'),
  googleClientId: z.string().min(1, 'Google Client ID is required'),
  googleClientSecret: z.string().min(1, 'Google Client Secret is required'),
})

type SetupFormData = z.infer<typeof SetupSchema>

const apiBase = import.meta.env.VITE_PUBLIC_URL || import.meta.env.VITE_API_URL || 'http://127.0.0.1:8090'

function Input({
  id,
  label,
  type = 'text',
  error,
  registration,
}: {
  id: string
  label: string
  type?: string
  error?: string
  registration: UseFormRegisterReturn
}) {
  return (
    <label htmlFor={id} className="block">
      <span className="mb-1 block text-xs font-medium text-gray-500">{label}</span>
      <input id={id} type={type} autoComplete="off" className={inputClass} aria-invalid={!!error} {...registration} />
      {error && <span className="mt-1 block text-xs text-youtube-700">{error}</span>}
    </label>
  )
}

function ProviderSection({ service, title, redirect, children }: { service: string; title: string; redirect: string; children: React.ReactNode }) {
  return (
    <section className="space-y-3">
      <h2 className="flex items-center gap-2 text-sm font-semibold text-ink">
        <ServiceIcon service={service} className="h-4 w-4" />
        {title}
      </h2>
      {children}
      <p className="text-xs text-gray-500">
        Redirect URI: <code className="rounded bg-gray-100 px-1 py-0.5 text-[11px] text-ink">{redirect}</code>
      </p>
    </section>
  )
}

function SetupWizard() {
  const navigate = useNavigate()
  const [error, setError] = useState<string | null>(null)
  const {
    register,
    handleSubmit,
    formState: { errors, isSubmitting },
  } = useForm<SetupFormData>({ resolver: zodResolver(SetupSchema) })

  const onSubmit = async (data: SetupFormData) => {
    setError(null)
    try {
      await setupAPI.save({
        spotify_client_id: data.spotifyId,
        spotify_client_secret: data.spotifySecret,
        google_client_id: data.googleClientId,
        google_client_secret: data.googleClientSecret,
      })
      navigate({ to: '/setup/success' })
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Something went wrong saving the credentials.')
    }
  }

  return (
    <main className="mx-auto flex min-h-screen max-w-lg flex-col justify-center px-4 py-12">
      <div className="mb-6 flex items-center gap-3">
        <SpotubeMark className="h-9 w-9" />
        <div>
          <h1 className="text-xl font-semibold text-ink">Set up Spotube</h1>
          <p className="text-sm text-gray-500">Add the OAuth apps Spotube signs in with. They're stored in the local database.</p>
        </div>
      </div>
      <Card className="p-6">
        <form className="space-y-6" onSubmit={handleSubmit(onSubmit)} noValidate>
          <ProviderSection service="spotify" title="Spotify app" redirect={`${apiBase}/api/auth/spotify/callback`}>
            <Input id="spotifyId" label="Client ID" error={errors.spotifyId?.message} registration={register('spotifyId')} />
            <Input
              id="spotifySecret"
              label="Client secret"
              type="password"
              error={errors.spotifySecret?.message}
              registration={register('spotifySecret')}
            />
          </ProviderSection>
          <div className="border-t border-gray-200" />
          <ProviderSection service="youtube" title="Google OAuth client" redirect={`${apiBase}/api/auth/youtube/callback`}>
            <Input id="googleClientId" label="Client ID" error={errors.googleClientId?.message} registration={register('googleClientId')} />
            <Input
              id="googleClientSecret"
              label="Client secret"
              type="password"
              error={errors.googleClientSecret?.message}
              registration={register('googleClientSecret')}
            />
          </ProviderSection>
          {error && <Notice tone="error">{error}</Notice>}
          <Button type="submit" variant="primary" className="w-full" loading={isSubmitting} icon={<KeyRound className="h-4 w-4" />}>
            Save credentials
          </Button>
        </form>
      </Card>
      <p className="mt-4 text-center text-xs text-gray-500">Step-by-step instructions are in the README under “OAuth setup”.</p>
    </main>
  )
}

export const Route = createLazyFileRoute('/setup/')({
  component: SetupWizard,
})
