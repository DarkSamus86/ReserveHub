package gateway

import "net/http"

const openAPISpecJSON = `{
  "openapi": "3.0.3",
  "info": {
    "title": "ReserveHub API Gateway",
    "version": "1.0.0",
    "description": "Public HTTP API Gateway for Auth, Platform Administration (Admin), and Tenant Workspace (Admin & Workspace facades)."
  },
  "servers": [
    {
      "url": "/",
      "description": "Current API Gateway origin"
    }
  ],
  "tags": [
    { "name": "Health", "description": "Liveness and readiness probes" },
    { "name": "Auth", "description": "Authentication, invitations, sessions, and tenant memberships" },
    { "name": "Platform Admin — Tenants", "description": "Platform-level tenant provisioning and lifecycle management" },
    { "name": "Platform Admin — Providers", "description": "Platform-level provider creation, listing, and status management" },
    { "name": "Workspace — Tenant", "description": "Tenant-scoped company profile, settings, branding, and domains" },
    { "name": "Workspace — Providers", "description": "Tenant-owner operations on city-level providers" }
  ],
  "components": {
    "securitySchemes": {
      "bearerAuth": {
        "type": "http",
        "scheme": "bearer",
        "bearerFormat": "JWT",
        "description": "EdDSA JWT issued by auth-service (platform scope or tenant scope)"
      }
    },
    "schemas": {
      "ErrorResponse": {
        "type": "object",
        "required": ["code"],
        "properties": {
          "code": { "type": "string", "example": "forbidden" },
          "message": { "type": "string", "example": "platform administrator required" }
        }
      },
      "Provider": {
        "type": "object",
        "required": ["id", "tenant_id", "name", "city", "status", "created_at", "updated_at"],
        "properties": {
          "id": { "type": "string", "format": "uuid" },
          "tenant_id": { "type": "string", "format": "uuid" },
          "name": { "type": "string", "example": "Kinopark Astana" },
          "city": { "type": "string", "example": "Astana" },
          "status": { "type": "string", "enum": ["ACTIVE", "SUSPENDED"] },
          "created_at": { "type": "string", "format": "date-time" },
          "updated_at": { "type": "string", "format": "date-time" }
        }
      },
      "ProviderListResponse": {
        "type": "object",
        "required": ["providers"],
        "properties": {
          "providers": {
            "type": "array",
            "items": { "$ref": "#/components/schemas/Provider" }
          },
          "next_page_token": { "type": "string" }
        }
      }
    }
  },
  "paths": {
    "/health/live": {
      "get": {
        "tags": ["Health"],
        "summary": "Gateway liveness check",
        "responses": {
          "200": { "description": "Gateway process is alive" }
        }
      }
    },
    "/health/ready": {
      "get": {
        "tags": ["Health"],
        "summary": "Gateway and upstream readiness check",
        "responses": {
          "200": { "description": "Gateway, Auth, Admin, and Workspace are ready" },
          "503": { "description": "One or more upstream dependencies are unavailable" }
        }
      }
    },
    "/v1/auth/register": {
      "post": {
        "tags": ["Auth"],
        "summary": "Register a new user account",
        "requestBody": {
          "required": true,
          "content": {
            "application/json": {
              "schema": {
                "type": "object",
                "required": ["email", "password"],
                "properties": {
                  "email": { "type": "string", "format": "email" },
                  "password": { "type": "string", "minLength": 12, "maxLength": 72 }
                }
              }
            }
          }
        },
        "responses": {
          "201": { "description": "Account registered" },
          "400": { "description": "Invalid input" },
          "409": { "description": "Email already registered" },
          "429": { "description": "Rate limited" }
        }
      }
    },
    "/v1/auth/login": {
      "post": {
        "tags": ["Auth"],
        "summary": "Authenticate user and issue EdDSA access & refresh tokens",
        "requestBody": {
          "required": true,
          "content": {
            "application/json": {
              "schema": {
                "type": "object",
                "required": ["email", "password"],
                "properties": {
                  "email": { "type": "string", "format": "email" },
                  "password": { "type": "string" }
                }
              }
            }
          }
        },
        "responses": {
          "200": { "description": "Session created" },
          "401": { "description": "Invalid credentials" },
          "429": { "description": "Rate limited" }
        }
      }
    },
    "/v1/auth/invitations/accept": {
      "post": {
        "tags": ["Auth"],
        "summary": "Accept a one-time tenant invitation token",
        "requestBody": {
          "required": true,
          "content": {
            "application/json": {
              "schema": {
                "type": "object",
                "required": ["token"],
                "properties": {
                  "token": { "type": "string" },
                  "password": { "type": "string", "minLength": 12, "maxLength": 72, "description": "Required for new invited identities; omit for existing accounts" }
                }
              }
            }
          }
        },
        "responses": {
          "200": { "description": "Invitation accepted and membership activated" },
          "400": { "description": "Invalid request" },
          "401": { "description": "Invalid or expired invitation token" }
        }
      }
    },
    "/v1/auth/refresh": {
      "post": {
        "tags": ["Auth"],
        "summary": "Rotate refresh token and issue a new access token",
        "requestBody": {
          "required": true,
          "content": {
            "application/json": {
              "schema": {
                "type": "object",
                "required": ["refresh_token"],
                "properties": {
                  "refresh_token": { "type": "string" }
                }
              }
            }
          }
        },
        "responses": {
          "200": { "description": "Session refreshed" },
          "401": { "description": "Invalid or expired refresh token" }
        }
      }
    },
    "/v1/auth/logout": {
      "post": {
        "tags": ["Auth"],
        "summary": "Revoke refresh session",
        "requestBody": {
          "required": true,
          "content": {
            "application/json": {
              "schema": {
                "type": "object",
                "required": ["refresh_token"],
                "properties": {
                  "refresh_token": { "type": "string" }
                }
              }
            }
          }
        },
        "responses": {
          "204": { "description": "Session revoked" },
          "401": { "description": "Invalid refresh token" }
        }
      }
    },
    "/v1/auth/me": {
      "get": {
        "tags": ["Auth"],
        "summary": "Get current authenticated user profile and active memberships",
        "security": [{ "bearerAuth": [] }],
        "responses": {
          "200": { "description": "Current user profile" },
          "401": { "description": "Unauthenticated" }
        }
      }
    },
    "/v1/auth/tenants": {
      "get": {
        "tags": ["Auth"],
        "summary": "List active tenant memberships for the authenticated user",
        "security": [{ "bearerAuth": [] }],
        "responses": {
          "200": { "description": "Active tenant memberships" },
          "401": { "description": "Unauthenticated" }
        }
      }
    },
    "/v1/auth/tenant-session": {
      "post": {
        "tags": ["Auth"],
        "summary": "Exchange user bearer token for a 15-minute tenant-scoped JWT",
        "security": [{ "bearerAuth": [] }],
        "requestBody": {
          "required": true,
          "content": {
            "application/json": {
              "schema": {
                "type": "object",
                "required": ["tenant_id"],
                "properties": {
                  "tenant_id": { "type": "string", "format": "uuid" }
                }
              }
            }
          }
        },
        "responses": {
          "200": { "description": "Tenant-scoped session issued" },
          "401": { "description": "Unauthenticated" },
          "403": { "description": "No active membership in requested tenant" }
        }
      }
    },
    "/v1/auth/tenants/{tenant_id}/members": {
      "get": {
        "tags": ["Auth"],
        "summary": "List tenant members and invitations",
        "security": [{ "bearerAuth": [] }],
        "parameters": [
          { "name": "tenant_id", "in": "path", "required": true, "schema": { "type": "string", "format": "uuid" } }
        ],
        "responses": {
          "200": { "description": "Tenant members list" },
          "401": { "description": "Unauthenticated" },
          "403": { "description": "Cross-tenant or insufficient access" }
        }
      }
    },
    "/v1/auth/tenants/{tenant_id}/invitations": {
      "post": {
        "tags": ["Auth"],
        "summary": "Invite a member into the tenant (tenant_owner only)",
        "security": [{ "bearerAuth": [] }],
        "parameters": [
          { "name": "tenant_id", "in": "path", "required": true, "schema": { "type": "string", "format": "uuid" } }
        ],
        "requestBody": {
          "required": true,
          "content": {
            "application/json": {
              "schema": {
                "type": "object",
                "required": ["email", "permission_profile"],
                "properties": {
                  "email": { "type": "string", "format": "email" },
                  "permission_profile": { "type": "string", "enum": ["tenant_viewer", "tenant_owner"] }
                }
              }
            }
          }
        },
        "responses": {
          "201": { "description": "Invitation created" },
          "403": { "description": "Tenant owner required" }
        }
      }
    },
    "/v1/auth/tenants/{tenant_id}/members/{membership_id}/revoke": {
      "post": {
        "tags": ["Auth"],
        "summary": "Revoke a tenant membership (tenant_owner only)",
        "security": [{ "bearerAuth": [] }],
        "parameters": [
          { "name": "tenant_id", "in": "path", "required": true, "schema": { "type": "string", "format": "uuid" } },
          { "name": "membership_id", "in": "path", "required": true, "schema": { "type": "string", "format": "uuid" } }
        ],
        "requestBody": {
          "required": true,
          "content": {
            "application/json": {
              "schema": { "type": "object" }
            }
          }
        },
        "responses": {
          "200": { "description": "Membership revoked" },
          "403": { "description": "Tenant owner required" },
          "409": { "description": "Cannot revoke the last active owner" }
        }
      }
    },
    "/v1/admin/tenants": {
      "post": {
        "tags": ["Platform Admin — Tenants"],
        "summary": "Create a tenant and provision its initial owner",
        "security": [{ "bearerAuth": [] }],
        "parameters": [
          { "name": "Idempotency-Key", "in": "header", "required": true, "schema": { "type": "string", "format": "uuid" } }
        ],
        "requestBody": {
          "required": true,
          "content": {
            "application/json": {
              "schema": {
                "type": "object",
                "required": ["name", "slug", "owner_email", "timezone", "currency_code", "locale_code", "default_language", "plan_id"],
                "properties": {
                  "name": { "type": "string" },
                  "slug": { "type": "string" },
                  "owner_email": { "type": "string", "format": "email" },
                  "timezone": { "type": "string" },
                  "currency_code": { "type": "string" },
                  "locale_code": { "type": "string" },
                  "default_language": { "type": "string" },
                  "plan_id": { "type": "string", "format": "uuid" }
                }
              }
            }
          }
        },
        "responses": {
          "201": { "description": "Tenant provisioned" },
          "403": { "description": "Platform administrator required" },
          "409": { "description": "Conflict or idempotency key reuse" }
        }
      },
      "get": {
        "tags": ["Platform Admin — Tenants"],
        "summary": "List tenants with optional status filter and pagination",
        "security": [{ "bearerAuth": [] }],
        "parameters": [
          { "name": "page_size", "in": "query", "schema": { "type": "integer", "minimum": 1, "maximum": 200 } },
          { "name": "page_token", "in": "query", "schema": { "type": "string" } },
          { "name": "status", "in": "query", "schema": { "type": "string", "enum": ["PENDING", "ACTIVE", "SUSPENDED", "ARCHIVED"] } }
        ],
        "responses": {
          "200": { "description": "Paginated tenant list" },
          "403": { "description": "Platform administrator required" }
        }
      }
    },
    "/v1/admin/tenants/{tenant_id}": {
      "get": {
        "tags": ["Platform Admin — Tenants"],
        "summary": "Get administrative tenant detail card",
        "security": [{ "bearerAuth": [] }],
        "parameters": [
          { "name": "tenant_id", "in": "path", "required": true, "schema": { "type": "string", "format": "uuid" } }
        ],
        "responses": {
          "200": { "description": "Tenant detail card" },
          "403": { "description": "Platform administrator required" },
          "404": { "description": "Tenant not found" }
        }
      }
    },
    "/v1/admin/tenants/{tenant_id}/activate": {
      "post": {
        "tags": ["Platform Admin — Tenants"],
        "summary": "Activate a PENDING tenant (requires accepted owner invitation)",
        "security": [{ "bearerAuth": [] }],
        "parameters": [
          { "name": "tenant_id", "in": "path", "required": true, "schema": { "type": "string", "format": "uuid" } }
        ],
        "requestBody": {
          "required": true,
          "content": {
            "application/json": {
              "schema": {
                "type": "object",
                "properties": { "reason": { "type": "string" } }
              }
            }
          }
        },
        "responses": {
          "200": { "description": "Tenant activated" },
          "409": { "description": "Owner invitation not accepted or invalid transition" }
        }
      }
    },
    "/v1/admin/tenants/{tenant_id}/suspend": {
      "post": {
        "tags": ["Platform Admin — Tenants"],
        "summary": "Suspend an ACTIVE tenant",
        "security": [{ "bearerAuth": [] }],
        "parameters": [
          { "name": "tenant_id", "in": "path", "required": true, "schema": { "type": "string", "format": "uuid" } }
        ],
        "requestBody": {
          "required": true,
          "content": {
            "application/json": {
              "schema": {
                "type": "object",
                "properties": { "reason": { "type": "string" } }
              }
            }
          }
        },
        "responses": {
          "200": { "description": "Tenant suspended" }
        }
      }
    },
    "/v1/admin/tenants/{tenant_id}/resume": {
      "post": {
        "tags": ["Platform Admin — Tenants"],
        "summary": "Resume a SUSPENDED tenant to ACTIVE",
        "security": [{ "bearerAuth": [] }],
        "parameters": [
          { "name": "tenant_id", "in": "path", "required": true, "schema": { "type": "string", "format": "uuid" } }
        ],
        "requestBody": {
          "required": true,
          "content": {
            "application/json": {
              "schema": {
                "type": "object",
                "properties": { "reason": { "type": "string" } }
              }
            }
          }
        },
        "responses": {
          "200": { "description": "Tenant resumed" }
        }
      }
    },
    "/v1/admin/tenants/{tenant_id}/archive": {
      "post": {
        "tags": ["Platform Admin — Tenants"],
        "summary": "Archive a tenant (terminal state)",
        "security": [{ "bearerAuth": [] }],
        "parameters": [
          { "name": "tenant_id", "in": "path", "required": true, "schema": { "type": "string", "format": "uuid" } }
        ],
        "requestBody": {
          "required": true,
          "content": {
            "application/json": {
              "schema": {
                "type": "object",
                "properties": { "reason": { "type": "string" } }
              }
            }
          }
        },
        "responses": {
          "200": { "description": "Tenant archived" }
        }
      }
    },
    "/v1/admin/tenant-provisioning/{operation_id}": {
      "get": {
        "tags": ["Platform Admin — Tenants"],
        "summary": "Get tenant provisioning operation and live owner invitation status",
        "security": [{ "bearerAuth": [] }],
        "parameters": [
          { "name": "operation_id", "in": "path", "required": true, "schema": { "type": "string", "format": "uuid" } }
        ],
        "responses": {
          "200": { "description": "Provisioning operation status" },
          "404": { "description": "Operation not found" }
        }
      }
    },
    "/v1/admin/tenant-provisioning/{operation_id}/reissue-invitation": {
      "post": {
        "tags": ["Platform Admin — Tenants"],
        "summary": "Reissue an expired owner invitation",
        "security": [{ "bearerAuth": [] }],
        "parameters": [
          { "name": "operation_id", "in": "path", "required": true, "schema": { "type": "string", "format": "uuid" } }
        ],
        "requestBody": {
          "required": true,
          "content": {
            "application/json": {
              "schema": { "type": "object" }
            }
          }
        },
        "responses": {
          "202": { "description": "Invitation reissue queued" },
          "409": { "description": "Invitation is not expired or already accepted" }
        }
      }
    },
    "/v1/admin/tenants/{tenant_id}/providers": {
      "post": {
        "tags": ["Platform Admin — Providers"],
        "summary": "Create a city-level provider for an ACTIVE tenant",
        "security": [{ "bearerAuth": [] }],
        "parameters": [
          { "name": "tenant_id", "in": "path", "required": true, "schema": { "type": "string", "format": "uuid" } },
          { "name": "Idempotency-Key", "in": "header", "required": true, "schema": { "type": "string", "format": "uuid" } }
        ],
        "requestBody": {
          "required": true,
          "content": {
            "application/json": {
              "schema": {
                "type": "object",
                "required": ["name", "city"],
                "properties": {
                  "name": { "type": "string", "maxLength": 255, "example": "Kinopark Astana" },
                  "city": { "type": "string", "maxLength": 160, "example": "Astana" }
                }
              }
            }
          }
        },
        "responses": {
          "201": {
            "description": "Provider created",
            "content": { "application/json": { "schema": { "$ref": "#/components/schemas/Provider" } } }
          },
          "200": {
            "description": "Idempotent replay of existing provider creation",
            "content": { "application/json": { "schema": { "$ref": "#/components/schemas/Provider" } } }
          },
          "403": { "description": "Platform admin required or tenant is not ACTIVE" },
          "409": { "description": "Idempotency key reused with different payload" }
        }
      },
      "get": {
        "tags": ["Platform Admin — Providers"],
        "summary": "List providers belonging to a tenant (Platform Admin)",
        "security": [{ "bearerAuth": [] }],
        "parameters": [
          { "name": "tenant_id", "in": "path", "required": true, "schema": { "type": "string", "format": "uuid" } },
          { "name": "page_size", "in": "query", "schema": { "type": "integer", "minimum": 1, "maximum": 100 } },
          { "name": "page_token", "in": "query", "schema": { "type": "string" } }
        ],
        "responses": {
          "200": {
            "description": "Provider list",
            "content": { "application/json": { "schema": { "$ref": "#/components/schemas/ProviderListResponse" } } }
          },
          "403": { "description": "Platform administrator required" }
        }
      }
    },
    "/v1/admin/tenants/{tenant_id}/providers/{provider_id}": {
      "get": {
        "tags": ["Platform Admin — Providers"],
        "summary": "Get provider details by tenant and provider ID (Platform Admin)",
        "security": [{ "bearerAuth": [] }],
        "parameters": [
          { "name": "tenant_id", "in": "path", "required": true, "schema": { "type": "string", "format": "uuid" } },
          { "name": "provider_id", "in": "path", "required": true, "schema": { "type": "string", "format": "uuid" } }
        ],
        "responses": {
          "200": {
            "description": "Provider details",
            "content": { "application/json": { "schema": { "$ref": "#/components/schemas/Provider" } } }
          },
          "404": { "description": "Provider not found in tenant" }
        }
      }
    },
    "/v1/admin/tenants/{tenant_id}/providers/{provider_id}/status": {
      "patch": {
        "tags": ["Platform Admin — Providers"],
        "summary": "Change provider status (ACTIVE or SUSPENDED)",
        "security": [{ "bearerAuth": [] }],
        "parameters": [
          { "name": "tenant_id", "in": "path", "required": true, "schema": { "type": "string", "format": "uuid" } },
          { "name": "provider_id", "in": "path", "required": true, "schema": { "type": "string", "format": "uuid" } }
        ],
        "requestBody": {
          "required": true,
          "content": {
            "application/json": {
              "schema": {
                "type": "object",
                "required": ["status"],
                "properties": {
                  "status": { "type": "string", "enum": ["ACTIVE", "SUSPENDED"] }
                }
              }
            }
          }
        },
        "responses": {
          "200": {
            "description": "Updated provider",
            "content": { "application/json": { "schema": { "$ref": "#/components/schemas/Provider" } } }
          },
          "403": { "description": "Platform admin required or tenant is not ACTIVE" },
          "404": { "description": "Provider not found" }
        }
      }
    },
    "/v1/tenants/{tenant_id}": {
      "get": {
        "tags": ["Workspace — Tenant"],
        "summary": "Get company profile (tenant_owner or tenant_viewer)",
        "security": [{ "bearerAuth": [] }],
        "parameters": [
          { "name": "tenant_id", "in": "path", "required": true, "schema": { "type": "string", "format": "uuid" } }
        ],
        "responses": {
          "200": { "description": "Company profile" },
          "403": { "description": "Cross-tenant access denied" }
        }
      },
      "patch": {
        "tags": ["Workspace — Tenant"],
        "summary": "Update company name and slug (tenant_owner only)",
        "security": [{ "bearerAuth": [] }],
        "parameters": [
          { "name": "tenant_id", "in": "path", "required": true, "schema": { "type": "string", "format": "uuid" } }
        ],
        "requestBody": {
          "required": true,
          "content": {
            "application/json": {
              "schema": {
                "type": "object",
                "properties": {
                  "name": { "type": "string" },
                  "slug": { "type": "string" }
                }
              }
            }
          }
        },
        "responses": {
          "200": { "description": "Updated company profile" },
          "403": { "description": "Tenant owner required" }
        }
      }
    },
    "/v1/tenants/{tenant_id}/settings": {
      "get": {
        "tags": ["Workspace — Tenant"],
        "summary": "Get regional settings (tenant_owner or tenant_viewer)",
        "security": [{ "bearerAuth": [] }],
        "parameters": [
          { "name": "tenant_id", "in": "path", "required": true, "schema": { "type": "string", "format": "uuid" } }
        ],
        "responses": { "200": { "description": "Tenant settings" } }
      },
      "put": {
        "tags": ["Workspace — Tenant"],
        "summary": "Update regional settings (tenant_owner only)",
        "security": [{ "bearerAuth": [] }],
        "parameters": [
          { "name": "tenant_id", "in": "path", "required": true, "schema": { "type": "string", "format": "uuid" } }
        ],
        "requestBody": {
          "required": true,
          "content": {
            "application/json": {
              "schema": {
                "type": "object",
                "required": ["timezone", "currency_code", "locale_code", "default_language"],
                "properties": {
                  "timezone": { "type": "string" },
                  "currency_code": { "type": "string" },
                  "locale_code": { "type": "string" },
                  "default_language": { "type": "string" }
                }
              }
            }
          }
        },
        "responses": { "200": { "description": "Updated settings" } }
      }
    },
    "/v1/tenants/{tenant_id}/branding": {
      "get": {
        "tags": ["Workspace — Tenant"],
        "summary": "Get tenant branding (tenant_owner or tenant_viewer)",
        "security": [{ "bearerAuth": [] }],
        "parameters": [
          { "name": "tenant_id", "in": "path", "required": true, "schema": { "type": "string", "format": "uuid" } }
        ],
        "responses": { "200": { "description": "Tenant branding" } }
      },
      "put": {
        "tags": ["Workspace — Tenant"],
        "summary": "Update tenant branding (tenant_owner only)",
        "security": [{ "bearerAuth": [] }],
        "parameters": [
          { "name": "tenant_id", "in": "path", "required": true, "schema": { "type": "string", "format": "uuid" } }
        ],
        "requestBody": {
          "required": true,
          "content": {
            "application/json": {
              "schema": {
                "type": "object",
                "properties": {
                  "display_name": { "type": "string" },
                  "primary_color": { "type": "string" },
                  "secondary_color": { "type": "string" },
                  "accent_color": { "type": "string" }
                }
              }
            }
          }
        },
        "responses": { "200": { "description": "Updated branding" } }
      }
    },
    "/v1/tenants/{tenant_id}/domains": {
      "get": {
        "tags": ["Workspace — Tenant"],
        "summary": "List tenant domains (tenant_owner or tenant_viewer)",
        "security": [{ "bearerAuth": [] }],
        "parameters": [
          { "name": "tenant_id", "in": "path", "required": true, "schema": { "type": "string", "format": "uuid" } }
        ],
        "responses": { "200": { "description": "Tenant domains" } }
      }
    },
    "/v1/tenants/{tenant_id}/providers": {
      "get": {
        "tags": ["Workspace — Providers"],
        "summary": "List city-level providers of the company (tenant_owner only in Stage D)",
        "security": [{ "bearerAuth": [] }],
        "parameters": [
          { "name": "tenant_id", "in": "path", "required": true, "schema": { "type": "string", "format": "uuid" } },
          { "name": "page_size", "in": "query", "schema": { "type": "integer", "minimum": 1, "maximum": 100 } },
          { "name": "page_token", "in": "query", "schema": { "type": "string" } }
        ],
        "responses": {
          "200": {
            "description": "List of providers for the tenant",
            "content": { "application/json": { "schema": { "$ref": "#/components/schemas/ProviderListResponse" } } }
          },
          "403": { "description": "Tenant owner required or tenant is ARCHIVED" }
        }
      }
    },
    "/v1/tenants/{tenant_id}/providers/{provider_id}": {
      "get": {
        "tags": ["Workspace — Providers"],
        "summary": "Get provider profile (tenant_owner only in Stage D)",
        "security": [{ "bearerAuth": [] }],
        "parameters": [
          { "name": "tenant_id", "in": "path", "required": true, "schema": { "type": "string", "format": "uuid" } },
          { "name": "provider_id", "in": "path", "required": true, "schema": { "type": "string", "format": "uuid" } }
        ],
        "responses": {
          "200": {
            "description": "Provider profile",
            "content": { "application/json": { "schema": { "$ref": "#/components/schemas/Provider" } } }
          },
          "403": { "description": "Tenant owner required" },
          "404": { "description": "Provider not found in tenant" }
        }
      },
      "patch": {
        "tags": ["Workspace — Providers"],
        "summary": "Update provider display name or city (tenant_owner only, requires ACTIVE tenant)",
        "security": [{ "bearerAuth": [] }],
        "parameters": [
          { "name": "tenant_id", "in": "path", "required": true, "schema": { "type": "string", "format": "uuid" } },
          { "name": "provider_id", "in": "path", "required": true, "schema": { "type": "string", "format": "uuid" } }
        ],
        "requestBody": {
          "required": true,
          "content": {
            "application/json": {
              "schema": {
                "type": "object",
                "properties": {
                  "name": { "type": "string", "maxLength": 255 },
                  "city": { "type": "string", "maxLength": 160 }
                }
              }
            }
          }
        },
        "responses": {
          "200": {
            "description": "Updated provider profile",
            "content": { "application/json": { "schema": { "$ref": "#/components/schemas/Provider" } } }
          },
          "400": { "description": "Invalid name or city" },
          "403": { "description": "Tenant owner required or tenant is not ACTIVE" },
          "404": { "description": "Provider not found" }
        }
      }
    }
  }
}
`

const swaggerUIHTML = `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="utf-8" />
  <meta name="viewport" content="width=device-width, initial-scale=1" />
  <title>ReserveHub API Gateway — Swagger UI</title>
  <link rel="stylesheet" href="https://unpkg.com/swagger-ui-dist@5.18.2/swagger-ui.css" />
  <style>
    body { margin: 0; background: #f6f7f4; }
    .topbar { display: none; }
  </style>
</head>
<body>
  <div id="swagger-ui"></div>
  <script src="https://unpkg.com/swagger-ui-dist@5.18.2/swagger-ui-bundle.js" crossorigin></script>
  <script>
    window.addEventListener('load', function () {
      window.ui = SwaggerUIBundle({
        url: '/openapi.json',
        dom_id: '#swagger-ui',
        deepLinking: true,
        persistAuthorization: false
      });
    });
  </script>
</body>
</html>
`

func (g *Gateway) openAPISpec(w http.ResponseWriter, r *http.Request) {
	if !g.allowIP(w, r) {
		return
	}
	if r.URL.RawQuery != "" {
		writeError(w, http.StatusBadRequest, "invalid_request", "query parameters are not accepted on this endpoint")
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(openAPISpecJSON))
}

func (g *Gateway) swaggerUI(w http.ResponseWriter, r *http.Request) {
	if !g.allowIP(w, r) {
		return
	}
	if r.URL.RawQuery != "" {
		writeError(w, http.StatusBadRequest, "invalid_request", "query parameters are not accepted on this endpoint")
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(swaggerUIHTML))
}
