'use client';

import { useState, useEffect } from 'react';
import { getAvailableAppsWithInstallStatus, installApp, upgradeApp, AppWithInstallStatus } from '../../api/node';
import withAuth from '@/components/withAuth';
import { AppInstallation } from '@/types/node';

const AppStorePage = () => {
  const [apps, setApps] = useState<AppWithInstallStatus[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    const fetchApps = async () => {
      try {
        const availableApps = await getAvailableAppsWithInstallStatus();
        setApps(availableApps);
      } catch (err) {
        console.error('Error fetching apps:', err);
        setError('Error fetching apps. Please try again later.');
      } finally {
        setLoading(false);
      }
    };

    fetchApps();
  }, []);

  const handleInstallApp = async (app: AppWithInstallStatus) => {
    try {
      const result = await installApp(app.appRequest);
      console.log('App installed successfully:', result);
      // You might want to update the UI or state here to reflect the new installation
    } catch (err) {
      console.error('Error installing app:', err);
      setError('Error installing app. Please try again later.');
    }
  };

  const handleUpgradeApp = async (app: AppWithInstallStatus) => {
    try {
      const requestedAppInstallation = app.appRequest.app_installation;
      if (!requestedAppInstallation) {
        throw new Error('App installation not found');
      }

      const currentInstalledApp = app.installedApp;
      if (!currentInstalledApp) {
        throw new Error('Current installed app not found');
      }

      const reverseProxyRules = app.appRequest.reverse_proxy_rules;

      requestedAppInstallation.id = app.appID!;

      const result = await upgradeApp({ 
        app_installation: requestedAppInstallation, 
        reverse_proxy_rules: reverseProxyRules, 
        version: requestedAppInstallation.version 
      });
      if (result.success) {
        console.log('App upgraded successfully:', result);
      } else {
        console.error('Error upgrading app:', result.error);
        setError('Error upgrading app. Please try again later.');
      }
    } catch (err) {
      console.error('Error upgrading app:', err);
      setError('Error upgrading app. Please try again later.');
    }
  }

  if (loading) return <div className="flex justify-center items-center h-screen">Loading...</div>;
  if (error) return <div className="flex justify-center items-center h-screen">{error}</div>;

  return (

    <main className="flex flex-col w-full min-h-screen p-8">
      <h1 className="text-3xl font-bold mb-6">App Store</h1>
      <div> 
        {apps.map((app) => (
          <div key={app.appRequest.app_installation!.id} className="border rounded-lg p-4 bg-white">
            <h2 className="text-xl font-semibold mb-2">{app.appRequest.app_installation!.name}</h2>
            <p className="text-gray-600 mb-2">Installed Version: {app.installedApp?.version}</p>
            <p className="text-gray-600 mb-2">Availabler Version: {app.appRequest.app_installation!.version}</p>
            <p className="text-gray-600 mb-4">Driver: {app.appRequest.app_installation!.driver}</p>

            {/* TODO: Add an installing state to this button, and update automatically when done, ideally with a progress bar. */}
            {app.installed ? (
              app.needsUpdate ? (
                <button className="bg-yellow-500 text-white px-4 py-2 rounded hover:bg-yellow-600 transition-colors" onClick={() => handleUpgradeApp(app)}>
                  Update Available
                </button>
              ) : (
                <button className="bg-green-500 text-white px-4 py-2 rounded hover:bg-green-600 transition-colors" disabled>
                  Installed
                </button>
              )
            ) : (
              <button className="bg-blue-500 text-white px-4 py-2 rounded hover:bg-blue-600 transition-colors" onClick={() => handleInstallApp(app)}>
                Install
              </button>
            )}

          </div>
        ))}
      </div>
    </main>
  );
};

export default withAuth(AppStorePage);
