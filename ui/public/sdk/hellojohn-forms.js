class HelloJohnForm extends HTMLElement {
  constructor() {
    super();
    this.attachShadow({ mode: 'open' });
    this.state = {
      stepIndex: 0,
      formData: {},
      config: null,
      errors: {}
    };
  }

  static get observedAttributes() {
    return ['tenant', 'type'];
  }

  attributeChangedCallback(name, oldValue, newValue) {
    if (oldValue !== newValue) {
      this.init();
    }
  }

  async connectedCallback() {
    this.init();
  }

  async init() {
    const config = await this.fetchConfig();
    if (config) {
      // Normalize legacy config to steps
      if (!config.steps || config.steps.length === 0) {
        config.steps = [{
          id: 'default',
          title: 'Form',
          fields: config.fields || []
        }];
      }
      this.state.config = config;
      this.render();
    } else {
      this.shadowRoot.innerHTML = '<div style="color:red; padding: 1rem;">Error loading form configuration.</div>';
    }
  }

  async fetchConfig() {
    const tenant = this.getAttribute('tenant');
    const type = this.getAttribute('type') || 'login';
    if (!tenant) return null;

    try {
      const res = await fetch(`http://localhost:8081/v1/public/tenants/${tenant}/forms/${type}`);
      if (!res.ok) throw new Error('Failed to load form config');
      return await res.json();
    } catch (err) {
      console.error('HelloJohn SDK Error:', err);
      return null;
    }
  }

  validateStep() {
    const step = this.state.config.steps[this.state.stepIndex];
    const errors = {};
    let isValid = true;

    step.fields.forEach(field => {
      const value = this.state.formData[field.name] || '';

      // Required check
      if (field.required && !value.trim()) {
        errors[field.id] = 'This field is required';
        isValid = false;
      }
      // Min Length
      else if (field.minLength && value.length < field.minLength) {
        errors[field.id] = `Minimum ${field.minLength} characters required`;
        isValid = false;
      }
      // Max Length
      else if (field.maxLength && value.length > field.maxLength) {
        errors[field.id] = `Maximum ${field.maxLength} characters allowed`;
        isValid = false;
      }
      // Pattern
      else if (field.pattern && value) {
        try {
          const regex = new RegExp(field.pattern);
          if (!regex.test(value)) {
            errors[field.id] = 'Invalid format';
            isValid = false;
          }
        } catch (e) {
          console.warn('Invalid regex pattern:', field.pattern);
        }
      }
    });

    this.state.errors = errors;
    this.render();
    return isValid;
  }

  handleInput(e) {
    const { name, value } = e.target;
    this.state.formData[name] = value;
    // Clear error on input
    if (this.state.errors[e.target.id]) {
      delete this.state.errors[e.target.id];
      this.render();
    }
  }

  handleNext() {
    if (this.validateStep()) {
      this.state.stepIndex++;
      this.render();
    }
  }

  handleBack() {
    if (this.state.stepIndex > 0) {
      this.state.stepIndex--;
      this.render();
    }
  }

  handleSubmit(e) {
    e.preventDefault();
    if (this.validateStep()) {
      // Dispatch event
      this.dispatchEvent(new CustomEvent('hellojohn-submit', {
        detail: this.state.formData,
        bubbles: true,
        composed: true
      }));
      console.log('Form submitted:', this.state.formData);
    }
  }

  render() {
    const { config, stepIndex, formData, errors } = this.state;
    if (!config) return;

    const { theme, steps } = config;
    const currentStep = steps[stepIndex];
    const isLastStep = stepIndex === steps.length - 1;

    // --- Styles ---
    const style = `
        <style>
            :host {
                display: block;
                font-family: ${theme.fontFamily || 'system-ui, -apple-system, sans-serif'};
                --primary: ${theme.primaryColor};
                --bg: ${theme.backgroundColor};
                --text: ${theme.textColor || '#334155'};
                --radius: ${theme.borderRadius};
                --spacing: ${theme.spacing === 'compact' ? '0.75rem' : theme.spacing === 'relaxed' ? '1.5rem' : '1rem'};
            }
            .container {
                background-color: var(--bg);
                border-radius: var(--radius);
                padding: 2rem;
                box-shadow: 0 10px 15px -3px rgb(0 0 0 / 0.1);
                max-width: 400px;
                margin: 0 auto;
                color: var(--text);
            }
            .logo { text-align: center; margin-bottom: 1.5rem; }
            .logo img { height: 48px; object-fit: contain; }
            
            .step-indicator {
                display: flex;
                gap: 4px;
                margin-bottom: 1.5rem;
                justify-content: center;
            }
            .step-dot {
                height: 4px;
                flex: 1;
                background: #e2e8f0;
                border-radius: 2px;
                transition: background 0.3s;
            }
            .step-dot.active { background: var(--primary); }

            .step-title {
                font-size: 1.25rem;
                font-weight: 600;
                margin-bottom: 0.5rem;
                text-align: center;
                color: ${theme.headingColor || 'inherit'};
            }
            .step-desc {
                font-size: 0.875rem;
                color: #64748b;
                text-align: center;
                margin-bottom: 1.5rem;
            }

            .field { margin-bottom: var(--spacing); }
            label {
                display: block;
                font-size: 0.875rem;
                font-weight: 500;
                margin-bottom: 0.375rem;
            }
            input {
                width: 100%;
                padding: 0.625rem 0.875rem;
                border: 1px solid #cbd5e1;
                border-radius: ${theme.borderRadius === '9999px' ? '9999px' : '0.375rem'};
                font-size: 0.95rem;
                box-sizing: border-box;
                transition: all 0.2s;
                background: ${theme.inputStyle?.variant === 'filled' ? '#f1f5f9' : 'transparent'};
                border-width: ${theme.inputStyle?.variant === 'underlined' ? '0 0 1px 0' : '1px'};
                border-radius: ${theme.inputStyle?.variant === 'underlined' ? '0' : 'inherit'};
            }
            input:focus {
                outline: none;
                border-color: var(--primary);
                ring: ${theme.inputStyle?.variant === 'underlined' ? 'none' : '2px solid var(--primary)20'};
            }
            .error-msg {
                color: #ef4444;
                font-size: 0.75rem;
                margin-top: 0.25rem;
            }
            .help-text {
                color: #94a3b8;
                font-size: 0.75rem;
                margin-top: 0.25rem;
            }

            .actions {
                display: flex;
                gap: 0.75rem;
                margin-top: 2rem;
            }
            button {
                flex: 1;
                padding: 0.75rem;
                border: none;
                border-radius: ${theme.borderRadius === '9999px' ? '9999px' : '0.375rem'};
                font-size: 0.95rem;
                font-weight: 600;
                cursor: pointer;
                transition: opacity 0.2s;
            }
            .btn-primary {
                background-color: var(--primary);
                color: white;
            }
            .btn-secondary {
                background-color: #f1f5f9;
                color: #475569;
            }
            button:hover { opacity: 0.9; }
        </style>
        `;

    // --- Render Fields ---
    const fieldsHtml = currentStep.fields.map(field => `
            <div class="field">
                ${theme.showLabels ? `<label for="${field.id}">${field.label} ${field.required ? '<span style="color:red">*</span>' : ''}</label>` : ''}
                <input 
                    type="${field.type === 'phone' ? 'tel' : field.type}" 
                    id="${field.id}" 
                    name="${field.name}" 
                    placeholder="${field.placeholder || ''}"
                    value="${formData[field.name] || ''}"
                />
                ${errors[field.id] ? `<div class="error-msg">${errors[field.id]}</div>` : ''}
                ${field.helpText ? `<div class="help-text">${field.helpText}</div>` : ''}
            </div>
        `).join('');

    // --- Render Steps Indicator ---
    const stepsHtml = steps.length > 1 ? `
            <div class="step-indicator">
                ${steps.map((_, i) => `<div class="step-dot ${i <= stepIndex ? 'active' : ''}"></div>`).join('')}
            </div>
        ` : '';

    // --- Render Actions ---
    const backBtn = stepIndex > 0
      ? `<button type="button" class="btn-secondary" id="btn-back">Back</button>`
      : '';

    const nextBtn = isLastStep
      ? `<button type="submit" class="btn-primary">Submit</button>`
      : `<button type="button" class="btn-primary" id="btn-next">Next</button>`;

    this.shadowRoot.innerHTML = `
            ${style}
            <div class="container">
                ${theme.logoUrl ? `<div class="logo"><img src="${theme.logoUrl}" alt="Logo" /></div>` : ''}
                ${stepsHtml}
                
                <div class="step-header">
                    <div class="step-title">${currentStep.title}</div>
                    ${currentStep.description ? `<div class="step-desc">${currentStep.description}</div>` : ''}
                </div>

                <form id="hj-form">
                    ${fieldsHtml}
                    <div class="actions">
                        ${backBtn}
                        ${nextBtn}
                    </div>
                </form>
            </div>
        `;

    // --- Event Listeners ---
    const form = this.shadowRoot.getElementById('hj-form');
    form.addEventListener('submit', this.handleSubmit.bind(this));

    // Bind inputs
    form.querySelectorAll('input').forEach(input => {
      input.addEventListener('input', this.handleInput.bind(this));
    });

    // Bind buttons
    const btnNext = this.shadowRoot.getElementById('btn-next');
    if (btnNext) btnNext.addEventListener('click', this.handleNext.bind(this));

    const btnBack = this.shadowRoot.getElementById('btn-back');
    if (btnBack) btnBack.addEventListener('click', this.handleBack.bind(this));
  }
}

customElements.define('hellojohn-form', HelloJohnForm);
