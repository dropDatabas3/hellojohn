export const BASE_STYLES = `
body { font-family: 'Helvetica Neue', Helvetica, Arial, sans-serif; background-color: #f4f4f7; color: #333; margin: 0; padding: 0; -webkit-font-smoothing: antialiased; }
.container { width: 100%; max-width: 600px; margin: 0 auto; background-color: #ffffff; border-radius: 8px; overflow: hidden; box-shadow: 0 4px 12px rgba(0,0,0,0.05); }
.header { background-color: #1a1a1a; padding: 30px 40px; text-align: center; }
.header h1 { color: #ffffff; margin: 0; font-size: 24px; font-weight: 600; letter-spacing: -0.5px; }
.content { padding: 40px; line-height: 1.6; }
.button { display: inline-block; background-color: #0070f3; color: #ffffff; text-decoration: none; padding: 12px 30px; border-radius: 5px; font-weight: 600; margin-top: 20px; }
.footer { background-color: #f9f9f9; padding: 20px 40px; text-align: center; font-size: 12px; color: #999; border-top: 1px solid #eee; }
.info-box { background-color: #f0f7ff; border-left: 4px solid #0070f3; padding: 15px; margin: 20px 0; border-radius: 4px; font-size: 14px; color: #0056b3; }
.warning-box { background-color: #fff8f0; border-left: 4px solid #f5a623; padding: 15px; margin: 20px 0; border-radius: 4px; font-size: 14px; color: #8a5d00; }
`

const WRAPPER = (content: string) => `<!doctype html>
<html>
<head>
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<style>${BASE_STYLES}</style>
</head>
<body>
  <div style="padding: 40px 0;">
    <div class="container">
      <div class="header">
        <h1>{{.Tenant}}</h1>
      </div>
      <div class="content">
        ${content}
      </div>
      <div class="footer">
        <p>&copy; ${new Date().getFullYear()} {{.Tenant}}. Todos los derechos reservados.</p>
        <p>Este es un mensaje automático, por favor no respondas a este correo.</p>
      </div>
    </div>
  </div>
</body>
</html>`

export const DEFAULT_TEMPLATES = {
    verify_email: {
        subject: "Verifica tu correo electrónico",
        body: WRAPPER(`
        <h2 style="margin-top: 0; color: #333;">Bienvenido a {{.Tenant}}</h2>
        <p>Hola <strong>{{.UserEmail}}</strong>,</p>
        <p>Gracias por registrarte. Para comenzar, necesitamos confirmar que esta dirección de correo electrónico te pertenece.</p>
        <div style="text-align: center; margin: 30px 0;">
          <a href="{{.Link}}" class="button" style="color: #ffffff;">Verificar mi cuenta</a>
        </div>
        <p style="font-size: 14px; color: #666;">O copia y pega el siguiente enlace en tu navegador:</p>
        <p style="font-size: 12px; color: #888; background: #eee; padding: 10px; border-radius: 4px; word-break: break-all;">{{.Link}}</p>
        <div class="info-box">Este enlace caducará en <strong>{{.TTL}}</strong>.</div>
        <p>Si no creaste esta cuenta, puedes ignorar este mensaje.</p>
        `)
    },
    reset_password: {
        subject: "Restablecer tu contraseña",
        body: WRAPPER(`
        <h2 style="margin-top: 0; color: #333;">Recuperación de Contraseña</h2>
        <p>Hola <strong>{{.UserEmail}}</strong>,</p>
        <p>Hemos recibido una solicitud para restablecer la contraseña de tu cuenta en {{.Tenant}}.</p>
        <div style="text-align: center; margin: 30px 0;">
          <a href="{{.Link}}" class="button" style="background-color: #27272a; color: #ffffff;">Restablecer Contraseña</a>
        </div>
        <p>Si no solicitaste este cambio, por favor ignora este correo. Tu contraseña actual seguirá funcionando.</p>
        <div class="warning-box">Por seguridad, este enlace solo es válido por <strong>{{.TTL}}</strong>.</div>
        `)
    },
    user_blocked: {
        subject: "Aviso importante: Cuenta bloqueada",
        body: WRAPPER(`
        <h2 style="margin-top: 0; color: #d93025;">Acceso Restringido</h2>
        <p>Hola <strong>{{.UserEmail}}</strong>,</p>
        <p>Tu acceso a {{.Tenant}} ha sido suspendido temporalmente debido a una acción administrativa o de seguridad.</p>
        <div class="warning-box" style="border-color: #d93025; background-color: #fff5f5; color: #b71c1c;">
          <strong>Motivo:</strong> {{.Reason}}<br>
          <strong>Vigencia:</strong> Hasta {{.Until}}
        </div>
        <p>Si consideras que esto es un error, por favor ponte en contacto con nuestro equipo de soporte inmediatamente.</p>
        `)
    },
    user_unblocked: {
        subject: "Tu cuenta ha sido reactivada",
        body: WRAPPER(`
        <h2 style="margin-top: 0; color: #188038;">¡Estás de vuelta!</h2>
        <p>Hola <strong>{{.UserEmail}}</strong>,</p>
        <p>Nos complace informarte que las restricciones en tu cuenta de {{.Tenant}} han sido levantadas.</p>
        <p>Ya tienes acceso completo a todos nuestros servicios nuevamente.</p>
        <div style="text-align: center; margin: 30px 0;">
          <a href="#" class="button" style="background-color: #188038; color: #ffffff;">Iniciar Sesión</a>
        </div>
        <p>Gracias por tu paciencia.</p>
        `)
    }
}
