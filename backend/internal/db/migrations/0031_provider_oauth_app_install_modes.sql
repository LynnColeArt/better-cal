alter table integration_app_install_provider_exchanges
	drop constraint if exists integration_app_install_provider_exchanges_exchange_status_check;

alter table integration_app_install_provider_exchanges
	add constraint integration_app_install_provider_exchanges_exchange_status_check
	check (exchange_status in ('queued', 'blocked', 'credential_ready'));

alter table integration_app_install_provider_exchanges
	drop constraint if exists integration_app_install_provider_exchanges_exchange_mode_check;

alter table integration_app_install_provider_exchanges
	add constraint integration_app_install_provider_exchanges_exchange_mode_check
	check (exchange_mode in ('simulated', 'provider_oauth'));

alter table integration_app_install_completions
	drop constraint if exists integration_app_install_completions_completion_mode_check;

alter table integration_app_install_completions
	add constraint integration_app_install_completions_completion_mode_check
	check (completion_mode in ('simulated', 'provider_oauth'));

alter table integration_app_installations
	drop constraint if exists integration_app_installations_activation_mode_check;

alter table integration_app_installations
	add constraint integration_app_installations_activation_mode_check
	check (activation_mode in ('simulated', 'provider_oauth'));
