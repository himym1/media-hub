require 'yaml'

path = ARGV.fetch(0, 'api/openapi.yaml')
document = YAML.safe_load(File.read(path), aliases: true)
references = []
operation_ids = []

walk = lambda do |value|
  case value
  when Hash
    value.each do |key, child|
      references << child if key == '$ref' && child.is_a?(String)
      operation_ids << child if key == 'operationId' && child.is_a?(String)
      walk.call(child)
    end
  when Array
    value.each { |child| walk.call(child) }
  end
end
walk.call(document)

missing = references.uniq.reject do |reference|
  next false unless reference.start_with?('#/')

  reference.delete_prefix('#/').split('/').reduce(document) do |node, part|
    decoded = part.gsub('~1', '/').gsub('~0', '~')
    node.is_a?(Hash) ? node[decoded] : nil
  end
end

duplicates = operation_ids.group_by(&:itself).select { |_, values| values.length > 1 }.keys
abort("missing refs: #{missing.join(', ')}") unless missing.empty?
abort("duplicate operationIds: #{duplicates.join(', ')}") unless duplicates.empty?

puts "openapi-ok refs=#{references.uniq.length} operations=#{operation_ids.length}"
