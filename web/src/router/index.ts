import { createRouter, createWebHistory, type RouteRecordRaw } from "vue-router";
import { useAuthStore } from "@/stores/auth";

const routes: RouteRecordRaw[] = [
  {
    path: "/login",
    name: "login",
    component: () => import("@/views/LoginView.vue"),
    meta: { public: true },
  },
  {
    path: "/",
    component: () => import("@/components/layout/AppLayout.vue"),
    children: [
      {
        path: "",
        redirect: "/dashboard",
      },
      {
        path: "dashboard",
        name: "dashboard",
        component: () => import("@/views/DashboardView.vue"),
      },
      {
        path: "users",
        name: "users",
        component: () => import("@/views/UsersView.vue"),
      },
      {
        path: "keys",
        name: "keys",
        component: () => import("@/views/KeysView.vue"),
      },
      {
        path: "providers",
        name: "providers",
        component: () => import("@/views/ProvidersView.vue"),
      },
      {
        path: "usage",
        name: "usage",
        component: () => import("@/views/UsageLogsView.vue"),
      },
      {
        path: "settings",
        name: "settings",
        component: () => import("@/views/SettingsView.vue"),
      },
      {
        path: "error-rules",
        name: "error-rules",
        component: () => import("@/views/ErrorRulesView.vue"),
      },
      {
        path: "request-filters",
        name: "request-filters",
        component: () => import("@/views/RequestFiltersView.vue"),
      },
      {
        path: "sensitive-words",
        name: "sensitive-words",
        component: () => import("@/views/SensitiveWordsView.vue"),
      },
      {
        path: "notifications",
        name: "notifications",
        component: () => import("@/views/NotificationsView.vue"),
      },
      {
        path: "prices",
        name: "prices",
        component: () => import("@/views/PricesView.vue"),
      },
    ],
  },
  {
    path: "/big-screen",
    name: "big-screen",
    component: () => import("@/views/BigScreenView.vue"),
  },
  {
    path: "/:pathMatch(.*)*",
    name: "not-found",
    component: () => import("@/views/NotFoundView.vue"),
    meta: { public: true },
  },
];

const router = createRouter({
  history: createWebHistory(),
  routes,
});

router.beforeEach((to, _from, next) => {
  const auth = useAuthStore();
  if (to.meta?.public || to.name === "login") {
    next();
  } else if (!auth.isAuthenticated) {
    next({ name: "login" });
  } else {
    next();
  }
});

export default router;
