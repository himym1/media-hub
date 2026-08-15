#!/usr/bin/env ruby
# frozen_string_literal: true

require "yaml"

def require_condition(condition, message)
  abort(message) unless condition
end

path = File.expand_path("../deploy/compose.mikan-egress.yaml", __dir__)
compose = YAML.safe_load(File.read(path))
services = compose.fetch("services")
media_hub = services.fetch("media-hub")
egress = services.fetch("mikan-egress")

proxy = media_hub.fetch("environment").fetch("MEDIA_HUB_SOURCE_PROXY_URL")
require_condition(proxy.include?("mikan-egress:17898"), "Media Hub source proxy must use the owned egress service")
require_condition(media_hub.fetch("depends_on").key?("mikan-egress"), "Media Hub must wait for mikan-egress")
require_condition(egress.fetch("healthcheck").fetch("test").join(" ").include?("17898"), "mikan-egress healthcheck is required")
require_condition(egress.fetch("volumes").include?("./tunnel/id_ed25519:/run/secrets/mikan_egress_key:ro"), "mikan-egress key mount must stay read-only")
require_condition(!File.read(path).match?(/subx/i), "Media Hub egress compose must not depend on SubX")

layout = File.read(File.expand_path("../deploy/install-layout.sh", __dir__))
require_condition(layout.include?('"$root/tunnel"'), "install layout must create the tunnel directory")

puts "deploy-compose-ok"
