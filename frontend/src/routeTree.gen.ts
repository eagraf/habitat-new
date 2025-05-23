/* eslint-disable */

// @ts-nocheck

// noinspection JSUnusedGlobalSymbols

// This file was automatically generated by TanStack Router.
// You should NOT make any changes in this file as it will be overwritten.
// Additionally, you should also exclude this file from your linter and/or formatter to prevent it from being checked or modified.

// Import Routes

import { Route as rootRoute } from './routes/__root'
import { Route as ServerImport } from './routes/server'
import { Route as LoginImport } from './routes/login'
import { Route as AppStoreImport } from './routes/app-store'
import { Route as AddUserImport } from './routes/add-user'
import { Route as IndexImport } from './routes/index'

// Create/Update Routes

const ServerRoute = ServerImport.update({
  id: '/server',
  path: '/server',
  getParentRoute: () => rootRoute,
} as any)

const LoginRoute = LoginImport.update({
  id: '/login',
  path: '/login',
  getParentRoute: () => rootRoute,
} as any)

const AppStoreRoute = AppStoreImport.update({
  id: '/app-store',
  path: '/app-store',
  getParentRoute: () => rootRoute,
} as any)

const AddUserRoute = AddUserImport.update({
  id: '/add-user',
  path: '/add-user',
  getParentRoute: () => rootRoute,
} as any)

const IndexRoute = IndexImport.update({
  id: '/',
  path: '/',
  getParentRoute: () => rootRoute,
} as any)

// Populate the FileRoutesByPath interface

declare module '@tanstack/react-router' {
  interface FileRoutesByPath {
    '/': {
      id: '/'
      path: '/'
      fullPath: '/'
      preLoaderRoute: typeof IndexImport
      parentRoute: typeof rootRoute
    }
    '/add-user': {
      id: '/add-user'
      path: '/add-user'
      fullPath: '/add-user'
      preLoaderRoute: typeof AddUserImport
      parentRoute: typeof rootRoute
    }
    '/app-store': {
      id: '/app-store'
      path: '/app-store'
      fullPath: '/app-store'
      preLoaderRoute: typeof AppStoreImport
      parentRoute: typeof rootRoute
    }
    '/login': {
      id: '/login'
      path: '/login'
      fullPath: '/login'
      preLoaderRoute: typeof LoginImport
      parentRoute: typeof rootRoute
    }
    '/server': {
      id: '/server'
      path: '/server'
      fullPath: '/server'
      preLoaderRoute: typeof ServerImport
      parentRoute: typeof rootRoute
    }
  }
}

// Create and export the route tree

export interface FileRoutesByFullPath {
  '/': typeof IndexRoute
  '/add-user': typeof AddUserRoute
  '/app-store': typeof AppStoreRoute
  '/login': typeof LoginRoute
  '/server': typeof ServerRoute
}

export interface FileRoutesByTo {
  '/': typeof IndexRoute
  '/add-user': typeof AddUserRoute
  '/app-store': typeof AppStoreRoute
  '/login': typeof LoginRoute
  '/server': typeof ServerRoute
}

export interface FileRoutesById {
  __root__: typeof rootRoute
  '/': typeof IndexRoute
  '/add-user': typeof AddUserRoute
  '/app-store': typeof AppStoreRoute
  '/login': typeof LoginRoute
  '/server': typeof ServerRoute
}

export interface FileRouteTypes {
  fileRoutesByFullPath: FileRoutesByFullPath
  fullPaths: '/' | '/add-user' | '/app-store' | '/login' | '/server'
  fileRoutesByTo: FileRoutesByTo
  to: '/' | '/add-user' | '/app-store' | '/login' | '/server'
  id: '__root__' | '/' | '/add-user' | '/app-store' | '/login' | '/server'
  fileRoutesById: FileRoutesById
}

export interface RootRouteChildren {
  IndexRoute: typeof IndexRoute
  AddUserRoute: typeof AddUserRoute
  AppStoreRoute: typeof AppStoreRoute
  LoginRoute: typeof LoginRoute
  ServerRoute: typeof ServerRoute
}

const rootRouteChildren: RootRouteChildren = {
  IndexRoute: IndexRoute,
  AddUserRoute: AddUserRoute,
  AppStoreRoute: AppStoreRoute,
  LoginRoute: LoginRoute,
  ServerRoute: ServerRoute,
}

export const routeTree = rootRoute
  ._addFileChildren(rootRouteChildren)
  ._addFileTypes<FileRouteTypes>()

/* ROUTE_MANIFEST_START
{
  "routes": {
    "__root__": {
      "filePath": "__root.tsx",
      "children": [
        "/",
        "/add-user",
        "/app-store",
        "/login",
        "/server"
      ]
    },
    "/": {
      "filePath": "index.tsx"
    },
    "/add-user": {
      "filePath": "add-user.tsx"
    },
    "/app-store": {
      "filePath": "app-store.tsx"
    },
    "/login": {
      "filePath": "login.tsx"
    },
    "/server": {
      "filePath": "server.tsx"
    }
  }
}
ROUTE_MANIFEST_END */
