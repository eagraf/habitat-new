'use client';

import { useState, useEffect } from 'react';
import { installApp } from '../../api/node';
import withAuth from '@/components/withAuth';

const AppStorePage = () => {
  const [apps, setApps] = useState<any[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    const fetchApps = async () => {
      try {
        const response = await fetch('/habitat/api/app_store/available_apps');
        if (!response.ok) {
          throw new Error('Failed to fetch apps');
        }
        const data = await response.json();
        setApps(data);
      } catch (err) {
        setError('Error fetching apps. Please try again later.');
      } finally {
        setLoading(false);
      }
    };

    fetchApps();
  }, []);


  const handleInstallApp = async (app: any) => {
    try {
      const result = await installApp(app);
      console.log('App installed successfully:', result);
      // You might want to update the UI or state here to reflect the new installation
    } catch (err) {
      console.error('Error installing app:', err);
      setError('Error installing app. Please try again later.');
    }
  };



  if (loading) return <div className="flex justify-center items-center h-screen">Loading...</div>;
  if (error) return <div className="flex justify-center items-center h-screen">{error}</div>;

  return (

    <main className="flex flex-col w-full min-h-screen p-8">
      <h1 className="text-3xl font-bold mb-6">App Store</h1>
      <div> 
        {apps.map((app) => (
          <div key={app.app_installation.id} className="border rounded-lg p-4 bg-white">
            <h2 className="text-xl font-semibold mb-2">{app.app_installation.name}</h2>
            <p className="text-gray-600 mb-2">Version: {app.app_installation.version}</p>
            <p className="text-gray-600 mb-4">Driver: {app.app_installation.driver}</p>
            <button className="bg-blue-500 text-white px-4 py-2 rounded hover:bg-blue-600 transition-colors" onClick={() => handleInstallApp(app)}>
              Install
            </button>
          </div>
        ))}
      </div>
    </main>
  );
};

export default withAuth(AppStorePage);
