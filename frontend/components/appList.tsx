import React from 'react';

interface App {
  id: string;
  name: string;
  status: string;
  image: string;
  port_mappings: { [key: string]: number };
}

interface AppListProps {
  apps: App[];
}

interface Process {
  app_id: string;
  state: string;
}

interface AppListProps {
  apps: App[];
  processes: Process[];
  reverseProxyRules: ReverseProxyRule[];
}

const AppList: React.FC<AppListProps> = ({ apps, processes, reverseProxyRules }) => {
  return (
    <div className="app-list">
      <h2 className="text-xl font-bold mb-4">Habitat Apps</h2>
      {apps.length === 0 ? (
        <p>No apps found.</p>
      ) : (
        <ul className="space-y-4">
          {apps.map((app) => {
            const matchingProcess = processes.find(process => process.app_id === app.id);
            const state = matchingProcess ? matchingProcess.state : app.status;
            
            const matchingRules = reverseProxyRules.filter(rule => rule.app_id === app.id);
            
            return (
              <li key={app.id} className="bg-white p-4 rounded-lg shadow">
                <div className="font-semibold">{app.name}</div>
                <div>State: {state}</div>
                {matchingRules.length > 0 && (
                  <div className="mt-2">
                    <span className="font-semibold">Access URLs:</span>
                    <ul className="list-disc list-inside">
                      {matchingRules.map((rule, index) => (
                        <li key={index}>
                          <a href={`${window.location.origin}${rule.matcher}`} target="_blank" rel="noopener noreferrer" className="text-blue-500 hover:underline">
                            {rule.matcher}
                          </a>
                        </li>
                      ))}
                    </ul>
                  </div>
                )}
              </li>
            );
          })}
        </ul>
      )}
    </div>
  );
};

export default AppList;
