import { createLazyFileRoute, useNavigate } from '@tanstack/react-router'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { z } from 'zod'
import { useState } from 'react'

// Zod schema for form validation
const SetupSchema = z.object({
  spotifyId: z.string().min(1, 'Spotify Client ID is required'),
  spotifySecret: z.string().min(1, 'Spotify Client Secret is required'),
  googleClientId: z.string().min(1, 'Google Client ID is required'),
  googleClientSecret: z.string().min(1, 'Google Client Secret is required'),
})

type SetupFormData = z.infer<typeof SetupSchema>

function SetupWizard() {
  const navigate = useNavigate()
  const [isSubmitting, setIsSubmitting] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const {
    register,
    handleSubmit,
    formState: { errors },
  } = useForm<SetupFormData>({
    resolver: zodResolver(SetupSchema),
  })

  const onSubmit = async (data: SetupFormData) => {
    setIsSubmitting(true)
    setError(null)

    try {
      const response = await fetch('/api/setup', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({
          spotify_id: data.spotifyId,
          spotify_secret: data.spotifySecret,
          google_client_id: data.googleClientId,
          google_client_secret: data.googleClientSecret,
        }),
      })

      if (!response.ok) {
        const errorText = await response.text()
        throw new Error(errorText || 'Failed to save credentials')
      }

      // Navigate to success page
      navigate({ to: '/setup/success' })
    } catch (err) {
      setError(err instanceof Error ? err.message : 'An error occurred')
    } finally {
      setIsSubmitting(false)
    }
  }

  return (
    <div className="min-h-screen bg-gray-50 flex items-center justify-center py-12 px-4 sm:px-6 lg:px-8">
      <div className="max-w-md w-full space-y-8">
        <div>
          <h2 className="mt-6 text-center text-3xl font-extrabold text-gray-900">
            Welcome to Spotube
          </h2>
          <p className="mt-2 text-center text-sm text-gray-600">
            Please configure your OAuth credentials to get started
          </p>
        </div>

        <form className="mt-8 space-y-6" onSubmit={handleSubmit(onSubmit)}>
          <div className="rounded-md shadow-sm space-y-4">
            {/* Spotify Credentials */}
            <div>
              <h3 className="text-lg font-medium text-gray-900 mb-4">Spotify Credentials</h3>
              <div className="space-y-4">
                <div>
                  <label htmlFor="spotifyId" className="block text-sm font-medium text-gray-700">
                    Spotify Client ID
                  </label>
                  <input
                    {...register('spotifyId')}
                    type="text"
                    className="mt-1 appearance-none relative block w-full px-3 py-2 border border-gray-300 placeholder-gray-500 text-gray-900 rounded-md focus:outline-none focus:ring-indigo-500 focus:border-indigo-500 sm:text-sm"
                    placeholder="Enter your Spotify Client ID"
                  />
                  {errors.spotifyId && (
                    <p className="mt-2 text-sm text-red-600">{errors.spotifyId.message}</p>
                  )}
                </div>

                <div>
                  <label htmlFor="spotifySecret" className="block text-sm font-medium text-gray-700">
                    Spotify Client Secret
                  </label>
                  <input
                    {...register('spotifySecret')}
                    type="password"
                    className="mt-1 appearance-none relative block w-full px-3 py-2 border border-gray-300 placeholder-gray-500 text-gray-900 rounded-md focus:outline-none focus:ring-indigo-500 focus:border-indigo-500 sm:text-sm"
                    placeholder="Enter your Spotify Client Secret"
                  />
                  {errors.spotifySecret && (
                    <p className="mt-2 text-sm text-red-600">{errors.spotifySecret.message}</p>
                  )}
                </div>
              </div>
            </div>

            {/* Google Credentials */}
            <div>
              <h3 className="text-lg font-medium text-gray-900 mb-4">Google Credentials</h3>
              <div className="space-y-4">
                <div>
                  <label htmlFor="googleClientId" className="block text-sm font-medium text-gray-700">
                    Google Client ID
                  </label>
                  <input
                    {...register('googleClientId')}
                    type="text"
                    className="mt-1 appearance-none relative block w-full px-3 py-2 border border-gray-300 placeholder-gray-500 text-gray-900 rounded-md focus:outline-none focus:ring-indigo-500 focus:border-indigo-500 sm:text-sm"
                    placeholder="Enter your Google Client ID"
                  />
                  {errors.googleClientId && (
                    <p className="mt-2 text-sm text-red-600">{errors.googleClientId.message}</p>
                  )}
                </div>

                <div>
                  <label htmlFor="googleClientSecret" className="block text-sm font-medium text-gray-700">
                    Google Client Secret
                  </label>
                  <input
                    {...register('googleClientSecret')}
                    type="password"
                    className="mt-1 appearance-none relative block w-full px-3 py-2 border border-gray-300 placeholder-gray-500 text-gray-900 rounded-md focus:outline-none focus:ring-indigo-500 focus:border-indigo-500 sm:text-sm"
                    placeholder="Enter your Google Client Secret"
                  />
                  {errors.googleClientSecret && (
                    <p className="mt-2 text-sm text-red-600">{errors.googleClientSecret.message}</p>
                  )}
                </div>
              </div>
            </div>
          </div>

          {error && (
            <div className="rounded-md bg-red-50 p-4">
              <div className="flex">
                <div className="ml-3">
                  <h3 className="text-sm font-medium text-red-800">Error</h3>
                  <div className="mt-2 text-sm text-red-700">
                    <p>{error}</p>
                  </div>
                </div>
              </div>
            </div>
          )}

          <div>
            <button
              type="submit"
              disabled={isSubmitting}
              className="group relative w-full flex justify-center py-2 px-4 border border-transparent text-sm font-medium rounded-md text-white bg-indigo-600 hover:bg-indigo-700 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-indigo-500 disabled:opacity-50 disabled:cursor-not-allowed"
            >
              {isSubmitting ? 'Saving...' : 'Save Credentials'}
            </button>
          </div>
        </form>
      </div>
    </div>
  )
}

export const Route = createLazyFileRoute('/setup/')({
  component: SetupWizard,
}) 