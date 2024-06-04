import '../styles/globals.css'
import { Metadata } from 'next'
import Providers from '@/components/Providers'
import Version from '@/components/Version'
import Link from 'next/link'
import Modal from '@/components/Modal'

export const metadata: Metadata = {
  title: 'Habitat',
  description: 'Welcome to Next.js',
}

export default function RootLayout({
  children,
  modal,
}: {
  children: React.ReactNode
  modal: React.ReactNode
}) {
  return (
    <html lang="en">
      <Providers>
        <body>
          <header className="flex px-6 py-4 justify-between">
            <nav className='flex gap-4'>
              <Link href="/">🌱</Link>
              <Link href="/test">Test Route</Link>
            </nav>
            <Version />
          </header>
          {children}
          <Modal>{modal}</Modal>
        </body>
      </Providers>
    </html>
  )
}
