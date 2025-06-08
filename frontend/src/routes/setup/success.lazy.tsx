import { createLazyFileRoute, useNavigate } from '@tanstack/react-router'
import { useEffect } from 'react'

function SetupSuccess() {
  const navigate = useNavigate()

  useEffect(() => {
    // Redirect to dashboard after 3 seconds
    const timer = setTimeout(() => {
      navigate({ to: '/dashboard' })
    }, 3000)

    return () => clearTimeout(timer)
  }, [navigate])

  return (
    <div className="min-h-screen bg-gray-50 flex items-center justify-center py-12 px-4 sm:px-6 lg:px-8">
      <div className="max-w-md w-full space-y-8">
        <div className="text-center">
          <div className="mx-auto h-12 w-12 text-green-600">
            <svg
              className="h-12 w-12"
              fill="none"
              stroke="currentColor"
              viewBox="0 0 24 24"
              xmlns="http://www.w3.org/2000/svg"
            >
              <path
                strokeLinecap="round"
                strokeLinejoin="round"
                strokeWidth={2}
                d="M5 13l4 4L19 7"
              />
            </svg>
          </div>
          <h2 className="mt-6 text-center text-3xl font-extrabold text-gray-900">
            Setup Complete!
          </h2>
          <p className="mt-2 text-center text-sm text-gray-600">
            Your credentials have been saved successfully.
          </p>
          <p className="mt-2 text-center text-sm text-gray-500">
            Redirecting to dashboard in 3 seconds...
          </p>
        </div>

        <div className="mt-8">
          <button
            onClick={() => navigate({ to: '/dashboard' })}
            className="group relative w-full flex justify-center py-2 px-4 border border-transparent text-sm font-medium rounded-md text-white bg-green-600 hover:bg-green-700 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-green-500"
          >
            Continue to Dashboard
          </button>
        </div>
      </div>
    </div>
  )
}

export const Route = createLazyFileRoute('/setup/success')({
  component: SetupSuccess,
}) 